package oidcdiscovery

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ruko1202/xhttp/dialguard"
	"github.com/stretchr/testify/require"
)

// countingServer answers every request with handler and counts the requests
// that reached it.
func countingServer(t *testing.T, handler http.HandlerFunc) (url string, hits *atomic.Int64) {
	t.Helper()

	hits = new(atomic.Int64)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	return srv.URL, hits
}

// The key-set client is what go-oidc fetches the JWKS with, at a URL the
// discovery document chose. These pin the three properties that make handing
// it that URL safe; each is invisible from the verifier, which only sees
// "keys fetched" or "keys not fetched".
func TestKeySetClient(t *testing.T) {
	t.Parallel()

	// On the production constructor: the loopback exemption of the test one
	// would make this pass vacuously.
	t.Run("refuses an internal address", func(t *testing.T) {
		t.Parallel()

		url, hits := countingServer(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"keys":[]}`))
		})

		resp, err := New().keysClient.Get(url)
		if resp != nil {
			_ = resp.Body.Close()
		}

		require.ErrorIs(t, err, dialguard.ErrBlockedAddress)
		require.Zero(t, hits.Load(), "the guard must refuse before the connection")
	})

	t.Run("does not follow a redirect", func(t *testing.T) {
		t.Parallel()

		target, targetHits := countingServer(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"keys":[]}`))
		})
		origin, _ := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target, http.StatusFound)
		})

		resp, err := NewAllowingLoopback(time.Second).keysClient.Get(origin)
		require.NoError(t, err)
		_ = resp.Body.Close()

		require.Equal(t, http.StatusFound, resp.StatusCode)
		require.Zero(t, targetHits.Load(), "a redirect must not be followed")
	})

	t.Run("bounds the response body", func(t *testing.T) {
		t.Parallel()

		url, _ := countingServer(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(strings.Repeat("x", maxResponseBytes+1024)))
		})

		resp, err := NewAllowingLoopback(time.Second).keysClient.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Len(t, body, maxResponseBytes)
	})
}
