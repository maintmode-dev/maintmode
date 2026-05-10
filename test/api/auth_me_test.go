//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/test/api/client/client/auth"
)

// TestAuthAPI_Me_Unauthorized covers the 401 branch when no Bearer token is supplied.
// The happy-path (200 with profile fields) is exercised by handler-level unit tests in
// internal/app/api/public/auth/me_test.go, since the integration harness has no helper
// for seeding a user that matches a synthetic JWT subject.
func TestAuthAPI_Me_Unauthorized(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	params := auth.NewGetAPIV1MeParams().WithContext(ctx)

	_, err := apiClient.Auth.GetAPIV1Me(params, nil)
	require.Error(t, err)
	require.Equal(t, http.StatusUnauthorized, extractErrorCode(t, err))
}
