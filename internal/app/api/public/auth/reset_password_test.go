package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"
)

// randomSuffix keeps each case on its own address: the suite runs -count 2
// against a shared database, so a reused literal would resolve to a row another
// pass created.
func randomSuffix() string {
	return uuid.NewString()
}

// doResetConfirm drives the real handler with the given JSON body.
func doResetConfirm(t *testing.T, impl *Implementation, body string) recordedResponse {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/password/reset/confirm", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	c := echotest.ContextConfig{Request: req, Response: rec}.ToContext(t)
	require.NoError(t, impl.ResetPassword(c))

	return recordedResponse{
		status:       rec.Code,
		body:         rec.Body.String(),
		cacheControl: rec.Header().Get(echo.HeaderCacheControl),
	}
}

// Every failure must answer alike. The password-policy case is the one worth
// the test: it is refused before the code is redeemed, so answering
// differently would tell a caller holding a guessed code that the code was
// right -- the more valuable of the two secrets.
func TestResetPassword_FailuresAreIndistinguishable(t *testing.T) {
	t.Parallel()

	impl := initImplWithOTPFloor(t, 0)

	const nonce = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa="

	cases := map[string]string{
		"unknown address": fmt.Sprintf(
			`{"email":"nobody-%s@example.com","code":"123456","session_nonce":%q,"new_password":"a-long-enough-password"}`,
			randomSuffix(), nonce),
		"password too short": fmt.Sprintf(
			`{"email":"nobody-%s@example.com","code":"123456","session_nonce":%q,"new_password":"short"}`,
			randomSuffix(), nonce),
		"non-numeric code": fmt.Sprintf(
			`{"email":"nobody-%s@example.com","code":"abcdef","session_nonce":%q,"new_password":"a-long-enough-password"}`,
			randomSuffix(), nonce),
		"missing new password": fmt.Sprintf(
			`{"email":"nobody-%s@example.com","code":"123456","session_nonce":%q}`,
			randomSuffix(), nonce),
		"malformed body": `{`,
	}

	responses := make(map[string]recordedResponse, len(cases))
	for name, body := range cases {
		responses[name] = doResetConfirm(t, impl, body)
	}

	names := make([]string, 0, len(responses))
	for name := range responses {
		names = append(names, name)
	}

	for i := range names {
		for j := i + 1; j < len(names); j++ {
			a, b := responses[names[i]], responses[names[j]]
			require.Equal(t, a.status, b.status,
				"%s and %s answer with different statuses", names[i], names[j])
			require.Equal(t, a.body, b.body,
				"%s and %s answer with different bodies", names[i], names[j])
			require.Equal(t, a.cacheControl, b.cacheControl,
				"%s and %s answer with different cache headers", names[i], names[j])
		}
	}

	for name, got := range responses {
		require.Equal(t, http.StatusUnauthorized, got.status, "case %q", name)
		require.Equal(t, "no-store", got.cacheControl, "case %q", name)
		lowered := strings.ToLower(got.body)
		require.NotContains(t, lowered, "password", "case %q leaks the policy", name)
		require.NotContains(t, lowered, "expired", "case %q leaks the code state", name)
	}
}

// The reset request answers 202 with a nonce whatever address it is given, so
// the response cannot be used to discover which accounts exist.
func TestRequestPasswordReset_AlwaysAccepts(t *testing.T) {
	t.Parallel()

	impl := initImplWithOTPFloor(t, 0)

	for _, address := range []string{
		"nobody-" + randomSuffix() + "@example.com",
		"not even an address",
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/password/reset/request",
			strings.NewReader(fmt.Sprintf(`{"email":%q}`, address)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		c := echotest.ContextConfig{Request: req, Response: rec}.ToContext(t)
		require.NoError(t, impl.RequestPasswordReset(c))

		require.Equal(t, http.StatusAccepted, rec.Code, "address %q", address)
		require.Contains(t, rec.Body.String(), "session_nonce", "address %q", address)
		require.Equal(t, "no-store", rec.Header().Get(echo.HeaderCacheControl))
	}
}
