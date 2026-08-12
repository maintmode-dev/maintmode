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
	// logBodies is off unless a caller opts in with WithBodyLogging. Bodies are the
	// one place redaction cannot help: their shape is arbitrary, so any secret they
	// carry is unrecognizable. The safe default is not to write them at all.
	logBodies bool
}

// RoundTrip logs the exchange without ever writing a secret, at any level.
//
// The span fields matter more than they look: xlog.WithOperationSpan attaches them
// to the LOGGER, so they reappear on every later record in this context — including
// the error below, which is emitted in production where the level is info. That is
// why the URL is masked rather than merely omitted. The field was previously
// req.RequestURI, which is always "" on a client request (net/http forbids setting
// it), so the obvious "fix" would have been req.URL.String() — and that ships the
// telegram bot token, which lives in the path, straight to prod.
func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, span := xlog.WithOperationSpan(req.Context(), "xhttpclient.transport.RoundTrip",
		xfield.String("host", req.Host),
		xfield.String("method", req.Method),
		xfield.String("url", safeURL(req.URL)),
	)
	defer span.End()

	// Before the dump, not after: WithBearerToken is registered as a before-hook,
	// so logging first would record a header map that does not yet hold the
	// Authorization actually sent. That is a diagnostic lie on its own, and it also
	// makes any "the token is redacted" test self-confirming — it would pass with
	// redaction switched off entirely.
	t.doBeforeRoundTrip(ctx, req)

	requestFields := []xfield.Field{
		xfield.String("protocol", req.Proto),
		xfield.Any("headers", redactHeaders(req.Header)),
	}
	if t.logBodies {
		requestFields = append(requestFields, xfield.String("body", dumpRequest(req)))
	}
	xlog.Debug(ctx, "sending request", requestFields...)

	resp, err := t.tr.RoundTrip(req)
	if err != nil {
		// Safe as-is: *url.Error, which embeds the full URL, is built by
		// http.Client.do ABOVE this transport. Here err is a bare *net.OpError with
		// no URL in it. Do not "harden" this line; do keep the telegram regression
		// test that covers the frame above.
		xlog.Error(ctx, "failed to execute request", xfield.Error(err))
		return nil, err
	}

	responseFields := []xfield.Field{
		xfield.String("protocol", resp.Proto),
		xfield.String("status", resp.Status),
		xfield.Any("headers", redactHeaders(resp.Header)),
	}
	if t.logBodies {
		responseFields = append(responseFields, xfield.String("body", dumpResponse(resp)))
	}
	xlog.Debug(ctx, "sent request", responseFields...)

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

// dumpRequest renders the request body for the log and puts it back, so the body
// still reaches the wire in full. Truncation applies to the RETURNED string only —
// sending a truncated request would turn a logging option into data corruption.
func dumpRequest(req *http.Request) string {
	body := dump(req.Body)

	req.Body = io.NopCloser(bytes.NewReader(body))

	return truncateForLog(body)
}

// dumpResponse is the response-side twin, with the same restore-then-truncate rule
// so the caller still reads the whole body.
//
// Unlike the request side it must CLOSE what it replaces. resp.Body is the network
// body, and the in-memory copy swapped in below is a NopCloser: without this the
// caller's Close() would be a no-op on a connection that stays open forever. The
// request body is different — net/http owns closing that one.
func dumpResponse(resp *http.Response) string {
	body := dump(resp.Body)

	if resp.Body != nil {
		_ = resp.Body.Close()
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))

	return truncateForLog(body)
}

// dump reads a body for logging. It takes an io.Reader, not an io.ReadCloser: it
// never closes what it is given, and closing is a decision its two callers make
// differently — the response side must close what it replaces, the request side
// must not.
func dump(r io.Reader) []byte {
	if r == nil {
		return nil
	}

	body, err := io.ReadAll(r)
	if err != nil {
		return nil
	}

	return body
}

// truncateForLog bounds a body dump to maxLoggedBodyBytes and marks it when cut, so
// a reader can tell a truncated payload from a short one.
func truncateForLog(body []byte) string {
	if len(body) <= maxLoggedBodyBytes {
		return string(body)
	}

	return string(body[:maxLoggedBodyBytes]) + truncationMarker
}
