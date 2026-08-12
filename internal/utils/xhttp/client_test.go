package xhttp

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestClient covers the round trip end to end: the caller gets the response body
// intact, and the server gets the request body intact.
//
// The second half is the point. This suite used to assert only dump(resp.Body)
// AFTER closing it, which passed for an accidental reason — dumpResponse read the
// body eagerly and swapped in a bytes.Reader that survives Close. It therefore could
// not tell a working client from one that never sends a body at all. Now the
// response is read before Close, and the handler records what actually arrived.
func TestClient(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(logger))

	respBody := []byte(`{"hello": "world"}`)

	var gotBody []byte
	var gotContentType string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotContentType = r.Header.Get("Content-Type")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(respBody)
	}))
	t.Cleanup(s.Close)

	client := NewClient()

	do := func(t *testing.T, req *http.Request) []byte {
		t.Helper()

		resp, err := client.Do(req)
		require.NoError(t, err)
		t.Cleanup(func() { _ = resp.Body.Close() })

		require.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		return body
	}

	t.Run("no body", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, http.NoBody)
		require.NoError(t, err)

		require.Equal(t, respBody, do(t, req))
		require.Empty(t, gotBody)
	})

	t.Run("with body", func(t *testing.T) {
		sent := []byte(`{"ping": "pong"}`)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, bytes.NewBuffer(sent))
		require.NoError(t, err)

		require.Equal(t, respBody, do(t, req))
		require.Equal(t, sent, gotBody, "the request body must reach the server untouched")
	})

	t.Run("with query", func(t *testing.T) {
		sent := []byte(`{"ping": "pong"}`)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, bytes.NewBuffer(sent))
		require.NoError(t, err)
		req.URL.RawQuery = "foo=bar"

		require.Equal(t, respBody, do(t, req))
		require.Equal(t, sent, gotBody)
	})

	t.Run("with formvalue", func(t *testing.T) {
		data := url.Values{}
		data.Set("foo", "bar")
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, strings.NewReader(data.Encode()))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		require.Equal(t, respBody, do(t, req))
		require.Equal(t, data.Encode(), string(gotBody))
		require.Equal(t, "application/x-www-form-urlencoded", gotContentType)
	})
}
