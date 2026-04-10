//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/test/api/client/client/auth"
)

func TestAuthAPI_OAuthCallback_InvalidState(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	params := auth.NewGetAPIV1LoginOauthGoogleCallbackParams().
		WithContext(ctx).
		WithState("invalid").
		WithCode("abc")

	_, err := apiClient.Auth.GetAPIV1LoginOauthGoogleCallback(params)
	require.Error(t, err)

	code := extractErrorCode(t, err)
	require.Equal(t, http.StatusBadRequest, code)
}
