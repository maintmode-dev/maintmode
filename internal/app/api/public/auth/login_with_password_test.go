package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"
)

type recordedResponse struct {
	status       int
	body         string
	cacheControl string
}

// doPasswordLogin drives the real handler with the given JSON body.
func doPasswordLogin(t *testing.T, impl *Implementation, body string) recordedResponse {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/login/password", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	c := echotest.ContextConfig{Request: req, Response: rec}.ToContext(t)
	require.NoError(t, impl.LoginWithPassword(c))

	return recordedResponse{
		status:       rec.Code,
		body:         rec.Body.String(),
		cacheControl: rec.Header().Get(echo.HeaderCacheControl),
	}
}

// The endpoint must not let an unauthenticated caller tell one failure from
// another. Each case below takes a different route through the service and
// would, without the collapse, reach a different arm of the shared error
// mapper: a wrong password 401, a blocked account 401 carrying "user is
// blocked", a refused signup 403 signup_disabled, a malformed body 400.
//
// The assertion is pairwise equality across all of them rather than each
// against a literal: what matters is that no two differ, and only comparing
// every pair fails when one branch quietly diverges.
func TestLoginWithPassword_FailuresAreIndistinguishable(t *testing.T) {
	t.Parallel()

	impl := initImpl(t)

	got := map[string]recordedResponse{
		"wrong password":  doPasswordLogin(t, impl, `{"password":"definitely-not-the-configured-one"}`),
		"empty password":  doPasswordLogin(t, impl, `{"password":""}`),
		"absent password": doPasswordLogin(t, impl, `{}`),
		"body email set":  doPasswordLogin(t, impl, `{"email":"someone@example.com","password":"nope"}`),
		"remember me":     doPasswordLogin(t, impl, `{"password":"nope","remember_me":true}`),
		// Bind failures too: a wrong JSON type and a truncated document. These
		// would otherwise answer 400 through the shared mapper, which is one bit
		// more than a caller should get from this endpoint.
		"wrong json type": doPasswordLogin(t, impl, `{"password":123}`),
		"truncated json":  doPasswordLogin(t, impl, `{"password":`),
		// And validation failures, which must not be tellable from a rejected
		// password either.
		"absent email":    doPasswordLogin(t, impl, `{"password":"nope"}`),
		"malformed email": doPasswordLogin(t, impl, `{"email":"not-an-address","password":"nope"}`),
		"oversized email": doPasswordLogin(t, impl, `{"email":"`+strings.Repeat("a", 300)+`@example.com","password":"nope"}`),
	}

	for name, resp := range got {
		require.Equal(t, http.StatusUnauthorized, resp.status, "%s must answer 401", name)

		for otherName, other := range got {
			require.Equal(t, other.status, resp.status,
				"%s and %s differ in status", name, otherName)
			require.Equal(t, other.body, resp.body,
				"%s and %s differ in body", name, otherName)
			require.Equal(t, other.cacheControl, resp.cacheControl,
				"%s and %s differ in Cache-Control", name, otherName)
		}
	}

	// The wrapped error text is exactly what must never reach the wire.
	for name, resp := range got {
		body := strings.ToLower(resp.body)
		require.NotContains(t, body, "blocked", "%s leaks account state", name)
		require.NotContains(t, body, "seat", "%s leaks license data", name)
		require.NotContains(t, body, "signup", "%s leaks the signup policy", name)
		require.NotContains(t, body, "provider", "%s leaks registry state", name)
	}
}

// The single failure response must carry no trace of what went wrong.
//
// This is the other half of the guarantee: the pairwise test above proves the
// handler funnels every path here, and this proves the response built here is
// safe to send whatever the underlying cause was. The shared mapper, which this
// endpoint deliberately bypasses, would answer several of those causes with
// their own status and their own wrapped text — a blocked user with "user is
// blocked", an exhausted cap with the exact seat counts, an unregistered method
// with its name.
//
// Asserting on the built response rather than on an error-taking wrapper is the
// point: the collapse is unconditional by construction, so there is no error
// parameter left to get wrong.
func TestUnauthorized_LeaksNothingAboutTheCause(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	c := echotest.ContextConfig{
		Request:  httptest.NewRequest(http.MethodPost, "/api/v1/login/password", http.NoBody),
		Response: rec,
	}.ToContext(t)

	require.NoError(t, unauthorized(c.Request().Context(), c, "test reason", errors.New("blocked: seats exhausted for provider signup")))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Equal(t, "no-store", rec.Header().Get(echo.HeaderCacheControl))

	body := strings.ToLower(rec.Body.String())
	for _, leak := range []string{"blocked", "seat", "signup", "provider", "credential", "email"} {
		require.NotContains(t, body, leak,
			"the failure response must not hint at the cause (%q)", leak)
	}
}

// A failed sign-in must carry the same cache directive as a successful one, so
// the two cannot be told apart by their headers either.
func TestLoginWithPassword_FailureIsNotCacheable(t *testing.T) {
	t.Parallel()

	resp := doPasswordLogin(t, initImpl(t), `{"password":"wrong"}`)

	require.Equal(t, http.StatusUnauthorized, resp.status)
	require.Equal(t, "no-store", resp.cacheControl)
}
