//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	authclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/auth"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestAuthAPIUsers_List(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

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

	t.Run("valid roles filter allowed", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)
		roles := []string{string(entity.RoleAdmin)}

		resp, err := apiClient.GetApiV1UsersListWithResponse(ctx, &authclient.GetApiV1UsersListParams{Roles: &roles})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	})

	t.Run("unknown roles filter is rejected", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)
		roles := []string{"superuser"}

		resp, err := apiClient.GetApiV1UsersListWithResponse(ctx, &authclient.GetApiV1UsersListParams{Roles: &roles})
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode())
	})
}

func TestAuthAPIUsers_Block(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

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
	ctx := ctxWithLogger(context.Background(), t)

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

// TestAuthAPIUsers_UpdateTags covers the authorization boundary of the admin
// tags patch, which handler-level tests structurally cannot reach: the check
// lives in the RequireScenario middleware and policy.csv, not in the handler.
func TestAuthAPIUsers_UpdateTags(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	tag := "@" + xuuid.NewString()[:8]

	t.Run("editor forbidden", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleEditor)

		resp, err := apiClient.PatchApiV1UsersIdWithResponse(ctx, uuid.New(),
			authclient.PatchApiV1UsersIdJSONRequestBody{TelegramTag: &tag})
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, resp.StatusCode())
	})

	t.Run("reviewer forbidden", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleReviewer)

		resp, err := apiClient.PatchApiV1UsersIdWithResponse(ctx, uuid.New(),
			authclient.PatchApiV1UsersIdJSONRequestBody{TelegramTag: &tag})
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, resp.StatusCode())
	})

	t.Run("admin patching unknown user gets not found", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)

		resp, err := apiClient.PatchApiV1UsersIdWithResponse(ctx, uuid.New(),
			authclient.PatchApiV1UsersIdJSONRequestBody{TelegramTag: &tag})
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode(), "unexpected status: %s", resp.Body)
	})

	t.Run("admin sending a reserved slack tag gets bad request", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)
		reserved := "@channel"

		resp, err := apiClient.PatchApiV1UsersIdWithResponse(ctx, uuid.New(),
			authclient.PatchApiV1UsersIdJSONRequestBody{SlackTag: &reserved})
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), "unexpected status: %s", resp.Body)
	})
}

// TestAuthAPIUsers_UpdateTags_HappyPath closes the seam the negative cases
// leave open: they all target a random uuid, so none of them ever reaches a
// write. Handler tests cover the write but bypass the authz middleware, and the
// e2e cases above cover the middleware but stop at 403/404 — between them, an
// admin successfully changing a real user's tag is never exercised end to end.
//
// This drives the whole stack once: routing, token, scenario check, handler,
// service, database, and the listing that reads it back.
func TestAuthAPIUsers_UpdateTags_HappyPath(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	admin := setupAuthTestClientWithRoles(entity.RoleAdmin)

	// Any real user will do; the admin themselves is the one we know exists.
	list, err := admin.GetApiV1UsersListWithResponse(ctx, &authclient.GetApiV1UsersListParams{})
	require.NoError(t, err)
	require.NotNil(t, list.JSON200)
	require.NotEmpty(t, lo.FromPtr(list.JSON200.Users), "the listing must contain the caller at least")

	target := lo.FromPtr(list.JSON200.Users)[0]
	targetID, err := uuid.Parse(lo.FromPtr(target.Id))
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, targetID)

	tag := "@" + xuuid.NewString()[:8]

	resp, err := admin.PatchApiV1UsersIdWithResponse(ctx, targetID,
		authclient.PatchApiV1UsersIdJSONRequestBody{TelegramTag: &tag})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)
	require.Equal(t, tag, lo.FromPtr(resp.JSON200.TelegramTag), "the response must echo the stored value")

	// Read it back through a different endpoint: the response could be built
	// from the request rather than from what was persisted.
	after, err := admin.GetApiV1UsersListWithResponse(ctx, &authclient.GetApiV1UsersListParams{})
	require.NoError(t, err)
	require.NotNil(t, after.JSON200)

	stored, found := lo.Find(lo.FromPtr(after.JSON200.Users), func(u authclient.ApimodelsUser) bool {
		return lo.FromPtr(u.Id) == targetID.String()
	})
	require.True(t, found, "the patched user must still be listed")
	require.Equal(t, tag, lo.FromPtr(stored.TelegramTag), "the tag must survive the round trip")
}
