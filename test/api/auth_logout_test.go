//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	authclient "github.com/ruko1202/maintmode/test/api/client/auth"
)

func TestAuthAPI_Logout_InvalidToken(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	params := &authclient.PostApiV1LogoutParams{
		Authorization: lo.ToPtr("Bearer invalid"),
	}

	resp, err := apiClient.PostApiV1LogoutWithResponse(ctx, params,
		authclient.PostApiV1LogoutJSONRequestBody{RefreshToken: lo.ToPtr("invalid")})
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode(), "unexpected status: %s", resp.Body)
}

// TestAuthAPI_Logout_BodyFallback_NoCookie verifies the body-fallback path: BFFs
// that don't share a cookie domain pass refresh_token in the JSON body, with no
// cookie attached. The handler must accept this and reject only on token validity,
// not on missing cookie.
func TestAuthAPI_Logout_BodyFallback_NoCookie(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	params := &authclient.PostApiV1LogoutParams{
		Authorization: lo.ToPtr("Bearer invalid-but-shaped-like-a-token"),
	}

	// No cookies are attached — the oapi-codegen client sends none unless told otherwise.
	resp, err := apiClient.PostApiV1LogoutWithResponse(ctx, params,
		authclient.PostApiV1LogoutJSONRequestBody{RefreshToken: lo.ToPtr("body-only-refresh")})
	require.NoError(t, err)

	// Token is invalid → 401. The key assertion is that we did NOT get a 400 about
	// missing refresh token (which would mean the body fallback was not consulted).
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode(), "unexpected status: %s", resp.Body)
}
