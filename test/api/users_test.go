//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	authclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/auth"
)

func TestAuthAPIUsers_List(t *testing.T) {
	ctx := context.Background()

	t.Run("admin allowed", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)

		resp, err := apiClient.GetApiV1UsersListWithResponse(ctx, &authclient.GetApiV1UsersListParams{})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
		require.NotNil(t, resp.JSON200)
	})

	t.Run("editor forbidden", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleEditor)

		resp, err := apiClient.GetApiV1UsersListWithResponse(ctx, &authclient.GetApiV1UsersListParams{})
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, resp.StatusCode())
	})

	t.Run("missing token unauthorized", func(t *testing.T) {
		apiClient := setupAuthTestClient()

		resp, err := apiClient.GetApiV1UsersListWithResponse(ctx, &authclient.GetApiV1UsersListParams{})
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode())
	})

	t.Run("valid role filter allowed", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)
		role := string(entity.RoleAdmin)

		resp, err := apiClient.GetApiV1UsersListWithResponse(ctx, &authclient.GetApiV1UsersListParams{Role: &role})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	})

	t.Run("unknown role filter is rejected", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)
		role := "superuser"

		resp, err := apiClient.GetApiV1UsersListWithResponse(ctx, &authclient.GetApiV1UsersListParams{Role: &role})
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode())
	})
}

func TestAuthAPIUsers_Block(t *testing.T) {
	ctx := context.Background()

	t.Run("editor forbidden", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleEditor)

		resp, err := apiClient.PostApiV1UsersIdBlockWithResponse(ctx, uuid.New())
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, resp.StatusCode())
	})

	t.Run("admin blocking unknown user gets not found", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)

		resp, err := apiClient.PostApiV1UsersIdBlockWithResponse(ctx, uuid.New())
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode(), "unexpected status: %s", resp.Body)
	})
}

func TestAuthAPIUsers_Unblock(t *testing.T) {
	ctx := context.Background()

	t.Run("editor forbidden", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleEditor)

		resp, err := apiClient.PostApiV1UsersIdUnblockWithResponse(ctx, uuid.New())
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, resp.StatusCode())
	})

	t.Run("admin unblocking unknown user gets not found", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)

		resp, err := apiClient.PostApiV1UsersIdUnblockWithResponse(ctx, uuid.New())
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode(), "unexpected status: %s", resp.Body)
	})
}
