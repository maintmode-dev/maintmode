//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	authclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/auth"
)

func TestAuthAPI_LogoutAll_InvalidAccessToken(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	params := &authclient.PostApiV1LogoutAllParams{
		Authorization: "Bearer invalid",
	}

	resp, err := apiClient.PostApiV1LogoutAllWithResponse(ctx, params)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode(), "unexpected status: %s", resp.Body)
}
