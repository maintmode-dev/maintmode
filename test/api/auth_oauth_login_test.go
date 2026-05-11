//go:build api

package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xhttp"

	"github.com/ruko1202/maintmode/test/api/client/client/auth"
)

func TestAuthAPI_OAuthLogin(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	originalURI := "/"
	noRedirectClient := xhttp.NewClient(
		xhttp.WithTimeout(5*time.Second),
		xhttp.WithoutRedirect(),
	)

	params := auth.NewGetAPIV1LoginOauthGoogleParams().
		WithContext(ctx).
		WithHTTPClient(noRedirectClient).
		WithOriginalURI(&originalURI)

	err := apiClient.Auth.GetAPIV1LoginOauthGoogle(params)
	require.Error(t, err)

	code := extractErrorCode(t, err)
	require.Equal(t, http.StatusTemporaryRedirect, code)
}
