package auth

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
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

// Every error the service can hand back must produce the same response.
//
// The cases above all take one code path (Authenticate fails), so on their own
// they prove almost nothing: the leaking rows are the ones the shared mapper
// answers differently — a blocked user with 401 carrying wrapped error text, a
// refused signup with 403 signup_disabled, an exhausted cap with 403 AND the
// exact seat counts, an unregistered method with 400 naming it. None of those
// is reachable through the real service today (bootstrap is always registered
// and exempt from the cap), and that is exactly why they are asserted here: an
// endpoint whose uniformity depends on decisions made in other files is one
// refactor away from leaking.
func TestLoginWithPassword_EveryServiceErrorLooksIdentical(t *testing.T) {
	t.Parallel()

	failures := map[string]error{
		"invalid credentials": apperr.ErrInvalidCredentials,
		"blocked user":        apperr.ErrUserBlocked,
		"signup refused":      apperr.ErrSignupDisabled,
		"seats exhausted":     apperr.ErrSeatsLimitExceeded,
		"unsupported method":  apperr.ErrUnsupportedProvider,
		"wrapped in context":  fmt.Errorf("issue token pair: %w", apperr.ErrUserBlocked),
		"unexpected internal": errors.New("something nobody anticipated"),
	}

	got := make(map[string]recordedResponse, len(failures))
	for name, failure := range failures {
		rec := httptest.NewRecorder()
		c := echotest.ContextConfig{
			Request:  httptest.NewRequest(http.MethodPost, "/api/v1/login/password", http.NoBody),
			Response: rec,
		}.ToContext(t)

		require.NoError(t, respondToLoginFailure(c.Request().Context(), c, failure))
		got[name] = recordedResponse{status: rec.Code, body: rec.Body.String()}
	}

	for name, resp := range got {
		require.Equal(t, http.StatusUnauthorized, resp.status, "%s must answer 401", name)

		for otherName, other := range got {
			require.Equal(t, other.status, resp.status, "%s and %s differ in status", name, otherName)
			require.Equal(t, other.body, resp.body, "%s and %s differ in body", name, otherName)
		}

		body := strings.ToLower(resp.body)
		require.NotContains(t, body, "blocked", "%s leaks account state", name)
		require.NotContains(t, body, "seat", "%s leaks license data", name)
		require.NotContains(t, body, "signup", "%s leaks the signup policy", name)
		require.NotContains(t, body, "provider", "%s leaks registry state", name)
	}
}

// The email must be bounded before it reaches the service, because on a
// credential mismatch it is what the audit record is attributed to — and
// audit_log.actor carries a btree index that errors above ~2704 bytes. An
// unauthenticated caller must not be able to write an arbitrarily large,
// arbitrarily shaped string there.
//
// This is asserted on the validator rather than on the response: every failure
// answers with the same 401 by design, so the indistinguishability tests above
// stay green whether the validation exists or not — verified by mutation. Only
// a direct assertion can fail when the rule is dropped.
func TestValidateLoginWithPasswordCmd_BoundsTheEmail(t *testing.T) {
	t.Parallel()

	base := func(email string) *entity.LoginWithPasswordCmd {
		return &entity.LoginWithPasswordCmd{Email: email, Password: "pw", ClientIP: "10.0.0.1"}
	}

	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{name: "a real address passes", email: "ops@example.com"},
		{name: "absent is rejected", email: "", wantErr: true},
		{name: "malformed is rejected", email: "not-an-address", wantErr: true},
		{name: "oversized is rejected", email: strings.Repeat("a", 300) + "@example.com", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateLoginWithPasswordCmd(t.Context(), base(tc.email))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
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
