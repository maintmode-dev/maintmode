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

func TestAuthAPI_Refresh_InvalidToken(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	params := auth.NewPostAPIV1RefreshParams().
		WithContext(ctx).
		WithRequest(&models.AuthRefreshTokenJSONRequest{RefreshToken: "invalid"})

	_, err := apiClient.Auth.PostAPIV1Refresh(params)
	require.Error(t, err)

	code := extractErrorCode(t, err)
	require.Equal(t, http.StatusUnauthorized, code)
}
