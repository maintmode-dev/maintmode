package xhttp

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Criterion 2: a header set directly on the request is redacted.
//
// Deliberately NOT set through WithBearerToken — see TestRedactsBearerTokenOption
// for why that distinction is load-bearing.
func TestRedactsAuthorizationHeader(t *testing.T) {
	t.Parallel()

	ctx, dump := observedContext(t)
	srvURL, _ := echoServer(t)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srvURL, http.NoBody)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+secret)

	resp, err := NewClient().Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	out := dump()
	assert.NotContains(t, out, secret)
	assert.Contains(t, out, "headers", "the headers field must survive redaction")
	assert.Contains(t, out, redacted)

	// The non-mutation guarantee, end to end through a real RoundTrip. In-place
	// redaction would strip Authorization from the request being sent, and that
	// surfaces as a runtime 401 against a live integration rather than as a red test.
	assert.Equal(t, "Bearer "+secret, req.Header.Get("Authorization"),
		"redaction must not mutate the caller's request")
}

// Criterion 3: a token injected by the WithBearerToken option is redacted too.
//
// This is the test that pins the CALL ORDER. WithBearerToken is a before-hook, so if
// the header dump runs above doBeforeRoundTrip the token is not in the map yet, and
// "the secret is absent from the log" becomes true for the wrong reason — it would
// hold with redaction switched off entirely.
//
// Asserting absence alone therefore proves nothing here. The request record must
// ALSO show Authorization present-and-redacted: that is what distinguishes "the
// dump saw the header and masked it" from "the dump ran too early and saw nothing".
// Moving the dump back above the hook turns the positive assertion below red.
func TestRedactsBearerTokenOption(t *testing.T) {
	t.Parallel()

	ctx, dump := observedContext(t)
	srvURL, seen := echoServer(t)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srvURL, http.NoBody)
	require.NoError(t, err)

	resp, err := NewClient(WithBearerToken(secret)).Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	out := dump()
	assert.NotContains(t, out, secret)

	// The request-side record must carry a redacted Authorization. Scoped to the
	// "sending request" line so a redacted RESPONSE header cannot satisfy it.
	sending, _, found := strings.Cut(out, "sent request")
	require.True(t, found, "both log records must be present")
	assert.Contains(t, sending, `"Authorization":"`+redacted+`"`,
		"the dump must run AFTER the before-hook, so it sees the header and masks it; "+
			"logging first records no Authorization at all and makes this test self-confirming")

	// Criterion 9: redaction must not disturb what is actually sent. Asserted on
	// the server side, because the wire is the only place this is observable — the
	// header is ADDED by the hook, so inspecting req.Header afterwards would be
	// asking a different question.
	assert.Equal(t, "Bearer "+secret, seen.header.Get("Authorization"),
		"the token must still reach the server intact")
}

// Response headers are redacted as well: Set-Cookie is response-side only, which is
// why the blocklist is a union of both directions.
func TestRedactsResponseSetCookie(t *testing.T) {
	t.Parallel()

	ctx, dump := observedContext(t)
	srvURL, _ := echoServer(t)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srvURL, http.NoBody)
	require.NoError(t, err)

	resp, err := NewClient().Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	out := dump()
	assert.NotContains(t, out, secret)
	assert.Contains(t, out, "Content-Type", "a harmless response header must survive")
}

// Criteria 4 and 6: a secret in the URL is masked wherever it sits, while the fields
// that make a log line useful survive.
//
// Both rows assert a surviving diagnostic alongside the absent secret, and that
// pairing is the point: an implementation that dropped the path or query wholesale
// would satisfy "the secret is absent" while destroying the diagnostics.
func TestRedactsSecretsInURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		survives string
		why      string
	}{
		{
			name:     "token in path",
			path:     "/bot" + secret + "/sendMessage",
			survives: http.MethodGet,
			why:      "the method must remain visible",
		},
		{
			name:     "token in query value",
			path:     "/x?access_token=" + secret,
			survives: "access_token",
			why:      "query keys carry the useful half of the diagnostics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, dump := observedContext(t)
			srvURL, _ := echoServer(t)

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, srvURL+tt.path, http.NoBody)
			require.NoError(t, err)

			resp, err := NewClient().Do(req)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())

			out := dump()
			assert.NotContains(t, out, secret)
			assert.Contains(t, out, tt.survives, tt.why)
		})
	}
}

// Criterion 5: the error path. This is the one that matters most in production —
// span fields ride on the logger, so they appear on this error record even though
// prod runs at info and never emits the debug lines above.
func TestErrorRecordCarriesNoSecret(t *testing.T) {
	t.Parallel()

	ctx, dump := observedContext(t)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		deadServerURL(t)+"/bot"+secret+"/sendMessage", http.NoBody)
	require.NoError(t, err)

	resp, err := NewClient().Do(req) //nolint:bodyclose // the request fails at dial, so there is no body
	require.Error(t, err)
	require.Nil(t, resp)

	out := dump()
	require.Contains(t, out, "failed to execute request",
		"the failure must still be logged — otherwise this test proves nothing")
	assert.NotContains(t, out, secret)
}
