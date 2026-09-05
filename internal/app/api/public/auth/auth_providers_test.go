package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"
)

func doListAuthMethods(t *testing.T, impl *Implementation) recordedResponse {
	t.Helper()

	rec := httptest.NewRecorder()
	c := echotest.ContextConfig{
		Request:  httptest.NewRequest(http.MethodGet, "/api/v1/auth/providers", http.NoBody),
		Response: rec,
	}.ToContext(t)

	require.NoError(t, impl.ListAuthMethods(c))

	return recordedResponse{status: rec.Code, body: rec.Body.String()}
}

// TestListAuthMethods_ReturnsExactlyTheContract asserts the response
// field-for-field rather than as a subset.
//
// A subset match is the wrong test for this endpoint. It is public and
// unauthenticated, so the risk is not a missing field but an extra one — a
// client id, an issuer URL, a count of configured providers — arriving because
// someone widened a struct upstream. Comparing the whole decoded body means any
// new field fails here and has to be added deliberately.
func TestListAuthMethods_ReturnsExactlyTheContract(t *testing.T) {
	t.Parallel()

	resp := doListAuthMethods(t, initImpl(t))
	require.Equal(t, http.StatusOK, resp.status)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(resp.body), &got))

	require.Equal(t, map[string]any{
		"methods": []any{
			map[string]any{"id": "email_password", "type": "password", "display_name": "Password"},
			map[string]any{"id": "email_otp", "type": "code", "display_name": "Email code"},
		},
	}, got)
}

// TestListAuthMethods_DoesNotVaryWithConfiguration pins the membership rule.
//
// email_password is unconditional, and asserting that against "a default
// configuration" would be a tautology. What is worth pinning is that the list
// does not move with the bootstrap block — the only configuration anyone might
// be tempted to gate it on. A running instance always has both halves of that
// block: an empty password means "generate one at startup", and an empty address
// fails boot outright.
func TestListAuthMethods_DoesNotVaryWithConfiguration(t *testing.T) {
	t.Parallel()

	first := doListAuthMethods(t, initImpl(t))
	second := doListAuthMethods(t, initImplWithOTPFloor(t, 0))

	require.Equal(t, first.body, second.body,
		"the method list must not depend on how the instance is configured")
}

// TestListAuthMethods_LeaksNoProviderConfiguration is the negative half. The
// values below are the ones an external-provider config carries, and none of
// them belongs in a public response.
func TestListAuthMethods_LeaksNoProviderConfiguration(t *testing.T) {
	t.Parallel()

	body := doListAuthMethods(t, initImpl(t)).body

	for _, forbidden := range []string{
		"client_id", "client_secret", "issuer", "http", "redirect_uri",
		"google", "bootstrap", "email@", "tenant",
	} {
		require.NotContains(t, body, forbidden,
			"the public method list must not carry %q", forbidden)
	}
}

// TestListAuthMethods_IsIdenticalForEveryCaller guards against the endpoint
// becoming a user-enumeration oracle. Nothing about the request may change the
// answer — an endpoint that responded differently to a known address would leak
// exactly what the sign-in endpoints work to hide.
func TestListAuthMethods_IsIdenticalForEveryCaller(t *testing.T) {
	t.Parallel()

	impl := initImpl(t)

	anonymous := doListAuthMethods(t, impl)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/providers?email=known@example.com", http.NoBody)
	req.Header.Set(echo.HeaderXRealIP, "203.0.113.1")
	req.Header.Set("Cookie", "session=whatever")
	c := echotest.ContextConfig{Request: req, Response: rec}.ToContext(t)
	require.NoError(t, impl.ListAuthMethods(c))

	require.Equal(t, anonymous.body, rec.Body.String())
	require.Equal(t, anonymous.status, rec.Code)
}
