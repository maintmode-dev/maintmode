package xhttp

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Criterion 7: with no opt-in there is no body field at all.
//
// Asserting the ABSENCE of the key, not an empty value: the old code always emitted
// "body", so a half-done change that logs an empty string would look identical to a
// request that genuinely had none.
func TestBodiesAreNotLoggedByDefault(t *testing.T) {
	t.Parallel()

	ctx, dump := observedContext(t)
	srvURL, _ := echoServer(t)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srvURL, strings.NewReader(`{"token":"`+secret+`"}`))
	require.NoError(t, err)

	resp, err := NewClient().Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	out := dump()
	assert.NotContains(t, out, secret, "the request body must not reach the log")
	assert.NotContains(t, out, `"body"`, "the body field must be absent entirely, not empty")
}

// Criterion 8, first half: the option actually works.
//
// Exercised through NewClient rather than by setting the struct field, because the
// option does a type assertion on *transport with a silent no-op fallthrough — it
// can quietly fail to engage, and a struct-level test would never notice.
func TestWithBodyLoggingEnablesBodies(t *testing.T) {
	t.Parallel()

	ctx, dump := observedContext(t)
	srvURL, _ := echoServer(t)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srvURL, strings.NewReader(`{"ping":"pong"}`))
	require.NoError(t, err)

	resp, err := NewClient(WithBodyLogging()).Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	out := dump()
	assert.Contains(t, out, `"body"`)
	assert.Contains(t, out, "ping", "the request body should be visible when explicitly enabled")
	assert.Contains(t, out, "ok", "the response body should be visible too")
}

// Criterion 8, second half: a body over the cap is cut and marked.
//
// Without this the "never log an unbounded body" boundary is prose that no test
// enforces — an implementation satisfying every other criterion could still write
// megabyte log lines.
func TestBodyLoggingTruncates(t *testing.T) {
	t.Parallel()

	ctx, dump := observedContext(t)
	srvURL, seen := echoServer(t)

	big := bytes.Repeat([]byte("A"), maxLoggedBodyBytes*2)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srvURL, bytes.NewReader(big))
	require.NoError(t, err)

	resp, err := NewClient(WithBodyLogging()).Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	out := dump()
	assert.Contains(t, out, truncationMarker, "an oversized dump must be marked as cut")

	// Criterion 10: truncation is a LOGGING concern. The server must still receive
	// every byte — otherwise a debug option silently corrupts real traffic.
	assert.Len(t, seen.body, len(big), "the full body must still reach the wire")
}

// Criterion 10 on the default path: bodies are not logged, and that must not change
// what the server receives either.
func TestRequestBodyReachesServerWhenNotLogged(t *testing.T) {
	t.Parallel()

	ctx, _ := observedContext(t)
	srvURL, seen := echoServer(t)

	sent := []byte(`{"ping":"pong"}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srvURL, bytes.NewReader(sent))
	require.NoError(t, err)

	resp, err := NewClient().Do(req)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	assert.Equal(t, sent, seen.body, "the request body must be intact")
	assert.NotEmpty(t, body, "the caller must still be able to read the response")
}
