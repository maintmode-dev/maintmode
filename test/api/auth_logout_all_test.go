//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/test/api/client/client/auth"
)

func TestAuthAPI_LogoutAll_InvalidAccessToken(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	params := auth.NewPostAPIV1LogoutAllParams().
		WithContext(ctx).
		WithAuthorization("Bearer invalid")

	_, err := apiClient.Auth.PostAPIV1LogoutAll(params)
	require.Error(t, err)

	code := extractErrorCode(t, err)
	require.Equal(t, http.StatusUnauthorized, code)
}
