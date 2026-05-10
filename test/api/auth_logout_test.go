//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/test/api/client/client/auth"
	"github.com/ruko1202/maintmode/test/api/client/models"
)

func TestAuthAPI_Logout_InvalidToken(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	authorization := "Bearer invalid"

	params := auth.NewPostAPIV1LogoutParams().
		WithContext(ctx).
		WithAuthorization(&authorization).
		WithRequest(&models.AuthRefreshTokenJSONRequest{RefreshToken: "invalid"})

	_, err := apiClient.Auth.PostAPIV1Logout(params)
	require.Error(t, err)

	code := extractErrorCode(t, err)
	require.Equal(t, http.StatusUnauthorized, code)
}

// TestAuthAPI_Logout_BodyFallback_NoCookie verifies the body-fallback path: BFFs
// that don't share a cookie domain pass refresh_token in the JSON body, with no
// cookie attached. The handler must accept this and reject only on token validity,
// not on missing cookie.
func TestAuthAPI_Logout_BodyFallback_NoCookie(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	authorization := "Bearer invalid-but-shaped-like-a-token"

	params := auth.NewPostAPIV1LogoutParams().
		WithContext(ctx).
		WithAuthorization(&authorization).
		WithRequest(&models.AuthRefreshTokenJSONRequest{RefreshToken: "body-only-refresh"})

	// No cookies are attached — go-openapi default transport sends none unless told otherwise.
	_, err := apiClient.Auth.PostAPIV1Logout(params)
	require.Error(t, err)

	// Token is invalid → 401. The key assertion is that we did NOT get a 400 about
	// missing refresh token (which would mean the body fallback was not consulted).
	require.Equal(t, http.StatusUnauthorized, extractErrorCode(t, err))
}
