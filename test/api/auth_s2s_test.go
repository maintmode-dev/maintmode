//go:build api

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ruko1202/xhttp/client"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	authclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/auth"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// s2sProtectedPaths are the token endpoints that only the BFF may call.
var s2sProtectedPaths = []string{
	"/api/v1/refresh",
	"/api/v1/login/password",
	"/api/v1/login/break-glass",
	"/api/v1/login/otp/verify",
	"/api/v1/login/oauth/code/exchange",
	"/api/v1/login/dev",
	"/api/v1/users/invitations/accept/password",
}

// s2sHTTPClient is shared by every raw call: a client per request leaves its
// keep-alive connection open until the idle timeout, and a few dozen of those
// exhaust the local port forwarder for the rest of the suite.
var s2sHTTPClient = client.NewClient(client.WithTimeout(5 * time.Second))

// postAuthRaw POSTs a body that no handler accepts to an auth route with the
// given s2s headers ("" = header absent) and returns the status and the
// response's error code. The unknown role is what makes the dev login refuse it
// instead of creating a user; every other handler ignores the field.
func postAuthRaw(ctx context.Context, t *testing.T, path, appName, token string) (status int, code string) {
	t.Helper()

	// A unique address keeps the per-address OTP limiter out of the way.
	body := fmt.Sprintf(`{"email":"s2s-%s@example.com","roles":["s2s-not-a-role"]}`, xuuid.NewString())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL("auth")+path, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	require.NoError(t, s2sEditor(appName, token)(ctx, req))

	resp, err := s2sHTTPClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var parsed struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(raw, &parsed)

	return resp.StatusCode, parsed.Code
}

func TestAuthAPI_S2S_RefusesUnauthenticatedClient(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	cases := []struct {
		name, appName, token string
	}{
		{name: "no headers"},
		{name: "unknown appname", appName: "not-the-bff", token: testAPIS2SToken},
		{name: "wrong token", appName: testAPIS2SAppName, token: strings.Repeat("x", len(testAPIS2SToken))},
	}

	for _, path := range s2sProtectedPaths {
		for _, tc := range cases {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				status, code := postAuthRaw(ctx, t, path, tc.appName, tc.token)
				require.Equal(t, http.StatusUnauthorized, status)
				require.Equal(t, "invalid_client", code)
			})
		}

		t.Run(path+"/the BFF's credential reaches the handler", func(t *testing.T) {
			_, code := postAuthRaw(ctx, t, path, testAPIS2SAppName, testAPIS2SToken)
			require.NotEqual(t, "invalid_client", code)
		})
	}
}

// Routes outside the token endpoints do not ask for the credential.
func TestAuthAPI_S2S_UnprotectedRoutesUnaffected(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	status, code := postAuthRaw(ctx, t, "/api/v1/login/otp/request", "", "")
	require.NotEqual(t, "invalid_client", code)
	require.NotEqual(t, http.StatusUnauthorized, status)

	resp, err := setupAuthTestClient().GetApiV1AuthProvidersWithResponse(ctx, s2sEditor("", ""))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)

	// The browser starts the OAuth dance itself, with no BFF in the path. It
	// shares the /login/oauth group with the guarded code/exchange, so the
	// guard must stay on that route and off the group.
	start, err := setupAuthTestClient().GetApiV1LoginOauthProviderStartWithResponse(ctx, "s2s-unknown-provider",
		&authclient.GetApiV1LoginOauthProviderStartParams{Binding: xuuid.NewString()}, s2sEditor("", ""))
	require.NoError(t, err)
	require.NotEqual(t, http.StatusUnauthorized, start.StatusCode(), "unexpected body: %s", start.Body)
}

// A refused client never reaches the refresh token: presenting the live token
// without the credential must leave it unrotated, so the BFF holding it can
// still rotate it for real. Had the refused call rotated it, the last call
// would land in the grace window and get no refresh_token back.
func TestAuthAPI_S2S_RefusedRefreshDoesNotRotate(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupAuthTestClient()

	login, err := apiClient.PostApiV1LoginDevWithResponse(ctx, devLoginBody())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, login.StatusCode(), "unexpected status: %s", login.Body)
	live := login.JSON200.RefreshToken

	refused, err := apiClient.PostApiV1RefreshWithResponse(ctx,
		authclient.PostApiV1RefreshJSONRequestBody{RefreshToken: live}, s2sEditor("", ""))
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, refused.StatusCode())
	require.Equal(t, "invalid_client", lo.FromPtr(refused.JSON401.Code))

	next, err := apiClient.PostApiV1RefreshWithResponse(ctx, authclient.PostApiV1RefreshJSONRequestBody{RefreshToken: live})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, next.StatusCode(), "unexpected status: %s", next.Body)
	require.NotEmpty(t, lo.FromPtr(next.JSON200.RefreshToken), "the refused call must not have rotated the token")
}
