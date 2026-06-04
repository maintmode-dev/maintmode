//go:build api

package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	authclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/auth"
	"github.com/ruko1202/maintmode/internal/utils/xhttp"
)

func TestAuthAPI_OAuthLogin(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	// The login endpoint issues a 307 redirect to the provider. The shared test
	// client follows redirects, so build a dedicated client backed by a
	// no-redirect HTTP client to observe the redirect status directly.
	noRedirectClient := xhttp.NewClient(
		xhttp.WithTimeout(5*time.Second),
		xhttp.WithoutRedirect(),
	)

	apiClient, err := authclient.NewClientWithResponses(
		baseURL("auth"),
		authclient.WithHTTPClient(noRedirectClient),
	)
	require.NoError(t, err)

	params := &authclient.GetApiV1LoginOauthGoogleParams{
		OriginalUri: lo.ToPtr("/"),
	}

	resp, err := apiClient.GetApiV1LoginOauthGoogleWithResponse(ctx, params)
	require.NoError(t, err)
	require.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode(), "unexpected status: %s", resp.Body)
}
