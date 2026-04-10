//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/test/api/client/client/auth"
)

func TestAuthAPI_OAuthLogin(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	originalURI := "/"
	params := auth.NewGetAPIV1LoginOauthGoogleParams().
		WithContext(ctx).
		WithOriginalURI(&originalURI)

	err := apiClient.Auth.GetAPIV1LoginOauthGoogle(params)
	require.Error(t, err)

	code := extractErrorCode(t, err)
	// In a configured environment handler redirects to OAuth provider (307).
	// go-openapi follows the redirect so the final response may be 200 from the OAuth provider.
	// If provider is not configured the handler may return internal error (500).
	require.Equal(t, http.StatusOK, code)
}
