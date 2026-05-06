package xhttp

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
)

type ClientOptions func(*http.Client)

func NewClient(opts ...ClientOptions) *http.Client {
	dialer := &net.Dialer{
		// Время на установку TCP соединения.
		// 5 секунд обычно хватает даже для межконтинентальных запросов.
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	client := &http.Client{
		// Общий лимит на весь запрос: от звонка до конца чтения Body.
		// 30 секунд — это безопасный максимум. Для быстрых API лучше 5-10с.
		Timeout: 30 * time.Second,
		Transport: &transport{
			beforeRoundTrip: make([]func(req *http.Request), 0),
			afterRoundTrip:  make([]func(req *http.Response), 0),
			tr: &http.Transport{
				Proxy:             http.ProxyFromEnvironment,
				ForceAttemptHTTP2: true,
				// Настройки пула соединений (чтобы не открывать новые на каждый чих)
				MaxIdleConns:        1000,
				MaxIdleConnsPerHost: 100,
				DialContext:         dialer.DialContext,
				// Время на обмен TLS-сертификатами.
				// Критично, если сеть "лагает".
				TLSHandshakeTimeout: 5 * time.Second,
				// Важно! Ограничивает время ожидания заголовков ответа.
				// Защищает от ситуаций, когда соединение есть, но сервер "думает" вечно.
				ResponseHeaderTimeout: 10 * time.Second,
				// Ожидание подтверждения от сервера перед отправкой большого тела (POST).
				ExpectContinueTimeout: 1 * time.Second,
				IdleConnTimeout:       90 * time.Second,
			},
		},
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

type transport struct {
	tr              *http.Transport
	beforeRoundTrip []func(req *http.Request)
	afterRoundTrip  []func(req *http.Response)
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, span := xlog.WithOperationSpan(req.Context(), "xhttpclient.transport.RoundTrip",
		xfield.String("host", req.Host),
		xfield.String("method", req.Method),
		xfield.String("request_uri", req.RequestURI),
	)
	defer span.End()

	xlog.Debug(ctx, "sending request",
		xfield.String("protocol", req.Proto),
		xfield.Any("query", req.URL.Query()),
		xfield.Any("headers", req.Header),
		xfield.String("body", dumpRequest(req)),
	)

	t.doBeforeRoundTrip(ctx, req)

	resp, err := t.tr.RoundTrip(req)
	if err != nil {
		xlog.Error(ctx, "failed to execute request", xfield.Error(err))
		return nil, err
	}

	xlog.Debug(ctx, "sent request",
		xfield.String("protocol", resp.Proto),
		xfield.String("status", resp.Status),
		xfield.Any("headers", resp.Header),
		xfield.String("body", dumpResponse(resp)),
	)

	t.doAfterRoundTrip(ctx, resp)

	return resp, err
}

func (t *transport) doBeforeRoundTrip(ctx context.Context, req *http.Request) {
	_, span := xlog.WithOperationSpan(ctx, "xhttpclient.transport.doBeforeRoundTrip")
	defer span.End()

	for _, f := range t.beforeRoundTrip {
		f(req)
	}
}

func (t *transport) doAfterRoundTrip(ctx context.Context, req *http.Response) {
	_, span := xlog.WithOperationSpan(ctx, "xhttpclient.transport.afterRoundTrip")
	defer span.End()

	for _, f := range t.afterRoundTrip {
		f(req)
	}
}

func dumpRequest(req *http.Request) string {
	body := dump(req.Body)

	req.Body = io.NopCloser(bytes.NewReader(body))

	return string(body)
}

func dumpResponse(resp *http.Response) string {
	body := dump(resp.Body)

	resp.Body = io.NopCloser(bytes.NewReader(body))

	return string(body)
}

func dump(r io.ReadCloser) []byte {
	if r == nil {
		return nil
	}

	body, err := io.ReadAll(r)
	if err != nil {
		return nil
	}

	return body
}
