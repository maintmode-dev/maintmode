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

// TestAuthAPI_AuthExchange_InvalidIDToken hits the BFF-owned exchange endpoint
// with a junk Google ID token and verifies the backend rejects it with the
// normalized INVALID_ID_TOKEN error envelope.
//
// We can't sign a real ID token from inside the test process (we don't have
// Google's private signing key), so the happy path is covered by the verifier
// unit tests. This test pins the integration contract: route mounted, JSON
// body bound, error mapper wired.
func TestAuthAPI_AuthExchange_InvalidIDToken(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	params := auth.NewPostAPIV1LoginOauthExchangeGoogleParams().
		WithContext(ctx).
		WithRequest(&models.ApiauthmodelsExchangeIDTokenRequest{
			IDToken: "this-is-not-a-valid-jwt",
		})

	_, err := apiClient.Auth.PostAPIV1LoginOauthExchangeGoogle(params)
	require.Error(t, err)
	require.Equal(t, http.StatusUnauthorized, extractErrorCode(t, err))
}

// TestAuthAPI_AuthExchange_MissingIDToken verifies the handler rejects an
// empty id_token before hitting the verifier.
func TestAuthAPI_AuthExchange_MissingIDToken(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	params := auth.NewPostAPIV1LoginOauthExchangeGoogleParams().
		WithContext(ctx).
		WithRequest(&models.ApiauthmodelsExchangeIDTokenRequest{
			IDToken: "",
		})

	_, err := apiClient.Auth.PostAPIV1LoginOauthExchangeGoogle(params)
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, extractErrorCode(t, err))
}
