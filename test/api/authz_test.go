//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	authclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/auth"
	maintmodeclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/maintmode"
)

func TestMaintmodeAPIAuth_MissingToken(t *testing.T) {
	ctx := context.Background()
	apiClient := setupMaintmodeTestClientWithoutAuth()

	resp, err := apiClient.GetApiV1ResourcesWithResponse(ctx, &maintmodeclient.GetApiV1ResourcesParams{})
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode())
}

func TestMaintmodeAPIAuth_InvalidToken(t *testing.T) {
	ctx := context.Background()
	apiClient := setupMaintmodeTestClientWithToken("invalid")

	resp, err := apiClient.GetApiV1ResourcesWithResponse(ctx, &maintmodeclient.GetApiV1ResourcesParams{})
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode())
}

func TestMaintmodeAPIRBAC_GuestReadAllowed(t *testing.T) {
	ctx := context.Background()
	apiClient := setupMaintmodeTestClientWithRoles(entity.RoleGuest)

	resp, err := apiClient.GetApiV1ResourcesWithResponse(ctx, &maintmodeclient.GetApiV1ResourcesParams{})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestMaintmodeAPIRBAC_GuestMutationForbidden(t *testing.T) {
	ctx := context.Background()
	apiClient := setupMaintmodeTestClientWithRoles(entity.RoleGuest)

	resp, err := apiClient.PostApiV1ResourceCreateWithResponse(ctx, maintmodeclient.PostApiV1ResourceCreateJSONRequestBody{
		Name:        lo.ToPtr("forbidden resource"),
		Description: lo.ToPtr("guest must not create resources"),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode())
}

func TestAuthAPIRBAC_GuestCanReadRoles(t *testing.T) {
	ctx := context.Background()
	apiClient := setupAuthTestClientWithRoles(entity.RoleGuest)

	resp, err := apiClient.GetApiV1RolesWithResponse(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestAuthAPIRBAC_GuestAuditForbidden(t *testing.T) {
	ctx := context.Background()
	apiClient := setupAuthTestClientWithRoles(entity.RoleGuest)
	limit := 1

	resp, err := apiClient.GetApiV1AuditLogWithResponse(ctx, &authclient.GetApiV1AuditLogParams{
		Limit: lo.ToPtr(limit),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode())
}

func TestAuthAPIRBAC_EditorAuditForbidden(t *testing.T) {
	ctx := context.Background()
	apiClient := setupAuthTestClientWithRoles(entity.RoleEditor)
	limit := 1

	resp, err := apiClient.GetApiV1AuditLogWithResponse(ctx, &authclient.GetApiV1AuditLogParams{
		Limit: lo.ToPtr(limit),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode())
}

func TestAuthAPIRBAC_ReviewerAuditForbidden(t *testing.T) {
	ctx := context.Background()
	apiClient := setupAuthTestClientWithRoles(entity.RoleReviewer)
	limit := 1

	resp, err := apiClient.GetApiV1AuditLogWithResponse(ctx, &authclient.GetApiV1AuditLogParams{
		Limit: lo.ToPtr(limit),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode())
}

func TestAuthAPIRBAC_AdminAuditAllowed(t *testing.T) {
	ctx := context.Background()
	apiClient := setupAuthTestClientWithRoles(entity.RoleAdmin)
	limit := 1

	resp, err := apiClient.GetApiV1AuditLogWithResponse(ctx, &authclient.GetApiV1AuditLogParams{
		Limit: lo.ToPtr(limit),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestAuthAPIRBAC_GuestRolesAssignForbidden(t *testing.T) {
	assertRolesAssignForbidden(t, entity.RoleGuest)
}

func TestAuthAPIRBAC_EditorRolesAssignForbidden(t *testing.T) {
	assertRolesAssignForbidden(t, entity.RoleEditor)
}

func TestAuthAPIRBAC_ReviewerRolesAssignForbidden(t *testing.T) {
	assertRolesAssignForbidden(t, entity.RoleReviewer)
}

func TestAuthAPIRBAC_GuestRolesRevokeForbidden(t *testing.T) {
	assertRolesRevokeForbidden(t, entity.RoleGuest)
}

func TestAuthAPIRBAC_EditorRolesRevokeForbidden(t *testing.T) {
	assertRolesRevokeForbidden(t, entity.RoleEditor)
}

func TestAuthAPIRBAC_ReviewerRolesRevokeForbidden(t *testing.T) {
	assertRolesRevokeForbidden(t, entity.RoleReviewer)
}

func assertRolesAssignForbidden(t *testing.T, role entity.Role) {
	t.Helper()
	ctx := context.Background()
	apiClient := setupAuthTestClientWithRoles(role)

	resp, err := apiClient.PostApiV1RolesAssignWithResponse(ctx, authclient.PostApiV1RolesAssignJSONRequestBody{
		UserId: lo.ToPtr("00000000-0000-0000-0000-000000000000"),
		Role:   lo.ToPtr(authclient.ApimodelsRole(entity.RoleEditor)),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode())
}

func assertRolesRevokeForbidden(t *testing.T, role entity.Role) {
	t.Helper()
	ctx := context.Background()
	apiClient := setupAuthTestClientWithRoles(role)

	resp, err := apiClient.PostApiV1RolesRevokeWithResponse(ctx, authclient.PostApiV1RolesRevokeJSONRequestBody{
		UserId: lo.ToPtr("00000000-0000-0000-0000-000000000000"),
		Role:   lo.ToPtr(authclient.ApimodelsRole(entity.RoleEditor)),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode())
}
