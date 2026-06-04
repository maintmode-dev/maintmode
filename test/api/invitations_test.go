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
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func uniqueInviteEmail() string {
	return xuuid.NewString() + "@api-invite-test.com"
}

func strPtr(s string) *string { return &s }

func TestAuthAPIInvitations_Invite(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	// Note: the happy-path create/revoke/resend/accept flows are covered by the
	// service-level integration tests in internal/services/invitation, which can
	// seed a real inviting admin (the invited_by_id FK requires a persisted
	// user). The API-level tests here focus on routing, authorization, request
	// validation, and the public preview contract — all reachable with a
	// synthetic admin token.

	t.Run("editor forbidden", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleEditor)

		resp, err := apiClient.PostApiV1UsersInviteWithResponse(ctx, authclient.PostApiV1UsersInviteJSONRequestBody{
			Email: strPtr(uniqueInviteEmail()),
			Roles: &[]string{string(entity.RoleEditor)},
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, resp.StatusCode())
	})

	t.Run("missing token unauthorized", func(t *testing.T) {
		apiClient := setupAuthTestClient()

		resp, err := apiClient.PostApiV1UsersInviteWithResponse(ctx, authclient.PostApiV1UsersInviteJSONRequestBody{
			Email: strPtr(uniqueInviteEmail()),
			Roles: &[]string{string(entity.RoleEditor)},
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode())
	})

	t.Run("empty roles rejected", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)

		resp, err := apiClient.PostApiV1UsersInviteWithResponse(ctx, authclient.PostApiV1UsersInviteJSONRequestBody{
			Email: strPtr(uniqueInviteEmail()),
			Roles: &[]string{},
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), "unexpected status: %s", resp.Body)
	})

	t.Run("invalid email rejected", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)

		resp, err := apiClient.PostApiV1UsersInviteWithResponse(ctx, authclient.PostApiV1UsersInviteJSONRequestBody{
			Email: strPtr("not-an-email"),
			Roles: &[]string{string(entity.RoleEditor)},
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), "unexpected status: %s", resp.Body)
	})
}

func TestAuthAPIInvitations_List(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	t.Run("admin allowed", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)

		resp, err := apiClient.GetApiV1UsersInvitationsWithResponse(ctx, &authclient.GetApiV1UsersInvitationsParams{})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
		require.NotNil(t, resp.JSON200)
	})

	t.Run("editor forbidden", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleEditor)

		resp, err := apiClient.GetApiV1UsersInvitationsWithResponse(ctx, &authclient.GetApiV1UsersInvitationsParams{})
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, resp.StatusCode())
	})

	t.Run("unknown status filter rejected", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)
		status := "bogus"

		resp, err := apiClient.GetApiV1UsersInvitationsWithResponse(ctx, &authclient.GetApiV1UsersInvitationsParams{Status: &status})
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode())
	})
}

func TestAuthAPIInvitations_Preview_Public(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	// Preview is public: no token required, and an unknown invitation token
	// returns 200 with status "invalid" — never 404, never leaking anything.
	apiClient := setupAuthTestClient()

	resp, err := apiClient.GetApiV1UsersInvitationsPreviewWithResponse(ctx, &authclient.GetApiV1UsersInvitationsPreviewParams{
		Token: "no-such-token",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)
	require.Equal(t, "invalid", *resp.JSON200.Status)
	require.Nil(t, resp.JSON200.SuggestedProvider)
}

func TestAuthAPIInvitations_RevokeResend(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	t.Run("revoke unknown invitation gets not found", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)

		resp, err := apiClient.PostApiV1UsersInvitationsIdRevokeWithResponse(ctx, uuid.New())
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode(), "unexpected status: %s", resp.Body)
	})

	t.Run("revoke as editor forbidden", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleEditor)

		resp, err := apiClient.PostApiV1UsersInvitationsIdRevokeWithResponse(ctx, uuid.New())
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, resp.StatusCode())
	})

	t.Run("resend unknown invitation gets not found", func(t *testing.T) {
		apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)

		resp, err := apiClient.PostApiV1UsersInvitationsIdResendWithResponse(ctx, uuid.New())
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode(), "unexpected status: %s", resp.Body)
	})
}
