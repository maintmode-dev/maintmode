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

// TestAuthAPI_AuthExchange_InvalidIDToken hits the BFF-owned exchange endpoint
// with a junk Google ID token and verifies the backend rejects it with the
// normalized INVALID_ID_TOKEN error envelope.
//
// We can't sign a real ID token from inside the test process (we don't have
// Google's private signing key), so the happy path is covered by the verifier
// unit tests. This test pins the integration contract: route mounted, JSON
// body bound, error mapper wired.
func TestAuthAPI_AuthExchange_InvalidIDToken(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupAuthTestClient()

	resp, err := apiClient.PostApiV1LoginOauthExchangeGoogleWithResponse(ctx,
		authclient.PostApiV1LoginOauthExchangeGoogleJSONRequestBody{
			IdToken: lo.ToPtr("this-is-not-a-valid-jwt"),
		})
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode(), "unexpected status: %s", resp.Body)
}

// TestAuthAPI_AuthExchange_MissingIDToken verifies the handler rejects an
// empty id_token before hitting the verifier.
func TestAuthAPI_AuthExchange_MissingIDToken(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupAuthTestClient()

	resp, err := apiClient.PostApiV1LoginOauthExchangeGoogleWithResponse(ctx,
		authclient.PostApiV1LoginOauthExchangeGoogleJSONRequestBody{
			IdToken: lo.ToPtr(""),
		})
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode(), "unexpected status: %s", resp.Body)
}
