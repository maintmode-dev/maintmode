//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAuthAPI_Me_Unauthorized covers the 401 branch when no Bearer token is supplied.
// The happy-path (200 with profile fields) is exercised by handler-level unit tests in
// internal/app/api/public/auth/me_test.go, since the integration harness has no helper
// for seeding a user that matches a synthetic JWT subject.
func TestAuthAPI_Me_Unauthorized(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	resp, err := apiClient.GetApiV1MeWithResponse(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode(), "unexpected status: %s", resp.Body)
}
