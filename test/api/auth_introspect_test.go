//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	authclient "github.com/ruko1202/maintmode/test/api/client/auth"
)

func TestAuthAPI_Introspect_WithoutS2SToken(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	resp, err := apiClient.PostApiV1S2sIntrospectWithResponse(ctx,
		authclient.PostApiV1S2sIntrospectJSONRequestBody{})
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode(), "unexpected status: %s", resp.Body)
}
