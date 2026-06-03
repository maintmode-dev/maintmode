//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	authclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/auth"
)

// TestAuthAPI_ConnectProvider_Unauthorized covers the 401 branch when no Bearer
// token is supplied. Happy-path and conflict/lockout branches are exercised by
// handler- and service-level unit tests, since the integration harness has no
// helper for seeding a user that matches a synthetic JWT subject.
func TestAuthAPI_ConnectProvider_Unauthorized(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	resp, err := apiClient.PostApiV1MeProvidersProviderConnectWithResponse(ctx,
		authclient.PostApiV1MeProvidersProviderConnectParamsProviderGoogle,
		authclient.PostApiV1MeProvidersProviderConnectJSONRequestBody{
			IdToken: lo.ToPtr("tok"),
		})
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode(), "unexpected status: %s", resp.Body)
}

// TestAuthAPI_DisconnectProvider_Unauthorized covers the 401 branch when no
// Bearer token is supplied.
func TestAuthAPI_DisconnectProvider_Unauthorized(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	resp, err := apiClient.DeleteApiV1MeProvidersProviderDisconnectWithResponse(ctx,
		authclient.DeleteApiV1MeProvidersProviderDisconnectParamsProviderGoogle)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode(), "unexpected status: %s", resp.Body)
}
