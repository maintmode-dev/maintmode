package xhttp

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestClient(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(logger))

	respBody := []byte(`{"hello": "world"}`)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(respBody)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(s.Close)

	client := NewClient()

	t.Run("no body", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, http.NoBody)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, respBody, dump(resp.Body))
	})

	t.Run("with body", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, bytes.NewBuffer([]byte(`{"ping": "pong"}`)))
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, respBody, dump(resp.Body))
	})

	t.Run("with query", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, bytes.NewBuffer([]byte(`{"ping": "pong"}`)))
		require.NoError(t, err)
		req.URL.RawQuery = "foo=bar"

		resp, err := client.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, respBody, dump(resp.Body))
	})

	t.Run("with formvalue", func(t *testing.T) {
		data := url.Values{}
		data.Set("foo", "bar")
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, strings.NewReader(data.Encode()))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := client.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, respBody, dump(resp.Body))
	})
}
