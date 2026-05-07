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
	noRedirectClient := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	params := auth.NewGetAPIV1LoginOauthGoogleParams().
		WithContext(ctx).
		WithHTTPClient(noRedirectClient).
		WithOriginalURI(&originalURI)

	err := apiClient.Auth.GetAPIV1LoginOauthGoogle(params)
	require.Error(t, err)

	code := extractErrorCode(t, err)
	require.Equal(t, http.StatusTemporaryRedirect, code)
}
