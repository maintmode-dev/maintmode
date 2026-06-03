//go:build api

package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	authclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/auth"
)

func TestAuthAPI_OAuthCallback_InvalidState(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	params := &authclient.GetApiV1LoginOauthGoogleCallbackParams{
		Code:  "abc",
		State: "invalid",
	}

	resp, err := apiClient.GetApiV1LoginOauthGoogleCallbackWithResponse(ctx, params)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode(), "unexpected status: %s", resp.Body)
}

// TestAuthAPI_OAuthCallback_JSONMode_InvalidState verifies the JSON-mode
// branch of the dual-mode callback returns a JSON error envelope (not HTML)
// when the request advertises Accept: application/json. The full happy path
// requires nonce + signed state cooperation that the API test harness can't
// produce without a real browser flow — that's covered by handler unit tests.
func TestAuthAPI_OAuthCallback_JSONMode_InvalidState(t *testing.T) {
	ctx := context.Background()

	url := fmt.Sprintf("http://%s:%s/auth/api/v1/login/oauth/google/callback?state=invalid&code=abc",
		viper.GetString(envAPIHost), viper.GetString(envAPIPort))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	require.NoError(t, err)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "body: %s", string(body))
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
}
