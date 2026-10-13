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

	"github.com/ruko1202/maintmode/internal/entity"
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
	"/api/v1/login/oauth/exchange/google",
	"/api/v1/users/invitations/accept/password",
}

// s2sHTTPClient is shared by every raw call: a client per request leaves its
// keep-alive connection open until the idle timeout, and a few dozen of those
// exhaust the local port forwarder for the rest of the suite.
var s2sHTTPClient = client.NewClient(client.WithTimeout(5 * time.Second))

// postAuthRaw POSTs a body that no handler accepts to an auth route with the
// given s2s headers ("" = header absent) and returns the status and the
// response's error code.
func postAuthRaw(ctx context.Context, t *testing.T, path, appName, token string) (status int, code string) {
	t.Helper()

	// A unique address keeps the per-address OTP limiter out of the way.
	body := fmt.Sprintf(`{"email":"s2s-%s@example.com"}`, xuuid.NewString())
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
	// shares the /login/oauth group with the guarded exchange/google, so the
	// guard must stay on that route and off the group.
	start, err := setupAuthTestClient().GetApiV1LoginOauthProviderStartWithResponse(ctx, "s2s-unknown-provider",
		&authclient.GetApiV1LoginOauthProviderStartParams{Binding: xuuid.NewString()}, s2sEditor("", ""))
	require.NoError(t, err)
	require.NotEqual(t, http.StatusUnauthorized, start.StatusCode(), "unexpected body: %s", start.Body)
}

// A refused client never reaches the refresh token: presenting an already
// rotated token without the credential must not count as reuse, so the family —
// and the successor the real BFF holds — survives.
func TestAuthAPI_S2S_RefusedRefreshDoesNotRevokeFamily(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupAuthTestClient()

	login, err := apiClient.PostApiV1LoginOauthExchangeGoogleWithResponse(ctx,
		authclient.PostApiV1LoginOauthExchangeGoogleJSONRequestBody{IdToken: lo.ToPtr("api-test-s2s-" + xuuid.NewString())},
		testRolesEditor(entity.RoleGuest),
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, login.StatusCode(), "unexpected status: %s", login.Body)
	first := login.JSON200.RefreshToken

	rotated, err := apiClient.PostApiV1RefreshWithResponse(ctx, authclient.PostApiV1RefreshJSONRequestBody{RefreshToken: first})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rotated.StatusCode(), "unexpected status: %s", rotated.Body)
	successor := rotated.JSON200.RefreshToken

	// Past the test stack's 1ms grace window: with the credential, presenting
	// `first` now would be reuse and revoke the family.
	time.Sleep(50 * time.Millisecond)

	refused, err := apiClient.PostApiV1RefreshWithResponse(ctx,
		authclient.PostApiV1RefreshJSONRequestBody{RefreshToken: first}, s2sEditor("", ""))
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, refused.StatusCode())
	require.Equal(t, "invalid_client", lo.FromPtr(refused.JSON401.Code))

	next, err := apiClient.PostApiV1RefreshWithResponse(ctx, authclient.PostApiV1RefreshJSONRequestBody{RefreshToken: successor})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, next.StatusCode(), "the family must survive a refused client: %s", next.Body)
}
