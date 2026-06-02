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

func TestAuthAPI_Refresh_InvalidToken(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	resp, err := apiClient.PostApiV1RefreshWithResponse(ctx,
		authclient.PostApiV1RefreshJSONRequestBody{RefreshToken: lo.ToPtr("invalid")})
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode(), "unexpected status: %s", resp.Body)
}
