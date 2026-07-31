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
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClientWithoutAuth()

	resp, err := apiClient.GetApiV1ResourcesWithResponse(ctx, &maintmodeclient.GetApiV1ResourcesParams{})
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode())
}

func TestMaintmodeAPIAuth_InvalidToken(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClientWithToken("invalid")

	resp, err := apiClient.GetApiV1ResourcesWithResponse(ctx, &maintmodeclient.GetApiV1ResourcesParams{})
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode())
}

func TestMaintmodeAPIRBAC_GuestReadAllowed(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClientWithRoles(entity.RoleGuest)

	resp, err := apiClient.GetApiV1ResourcesWithResponse(ctx, &maintmodeclient.GetApiV1ResourcesParams{})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestMaintmodeAPIRBAC_GuestMutationForbidden(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClientWithRoles(entity.RoleGuest)

	resp, err := apiClient.PostApiV1ResourceCreateWithResponse(ctx, maintmodeclient.PostApiV1ResourceCreateJSONRequestBody{
		Name:        lo.ToPtr("forbidden resource"),
		Description: lo.ToPtr("guest must not create resources"),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode())
}

func TestAuthAPIRBAC_GuestCanReadRoles(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupAuthTestClientWithRoles(entity.RoleGuest)

	resp, err := apiClient.GetApiV1RolesWithResponse(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestAuthAPIRBAC_GuestAuditForbidden(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupAuthTestClientWithRoles(entity.RoleGuest)
	limit := 1

	resp, err := apiClient.GetApiV1AuditLogWithResponse(ctx, &authclient.GetApiV1AuditLogParams{
		Limit: lo.ToPtr(limit),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode())
}

func TestAuthAPIRBAC_EditorAuditForbidden(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupAuthTestClientWithRoles(entity.RoleEditor)
	limit := 1

	resp, err := apiClient.GetApiV1AuditLogWithResponse(ctx, &authclient.GetApiV1AuditLogParams{
		Limit: lo.ToPtr(limit),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode())
}

func TestAuthAPIRBAC_ReviewerAuditForbidden(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupAuthTestClientWithRoles(entity.RoleReviewer)
	limit := 1

	resp, err := apiClient.GetApiV1AuditLogWithResponse(ctx, &authclient.GetApiV1AuditLogParams{
		Limit: lo.ToPtr(limit),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode())
}

func TestAuthAPIRBAC_AdminAuditAllowed(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
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
	ctx := ctxWithLogger(context.Background(), t)
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
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupAuthTestClientWithRoles(role)

	resp, err := apiClient.PostApiV1RolesRevokeWithResponse(ctx, authclient.PostApiV1RolesRevokeJSONRequestBody{
		UserId: lo.ToPtr("00000000-0000-0000-0000-000000000000"),
		Role:   lo.ToPtr(authclient.ApimodelsRole(entity.RoleEditor)),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode())
}

// The approvals page is gated on maintenance.approve rather than
// maintenance.read: guests can read maintenances, but a page called "awaiting my
// approval" is not a page they see empty — it is a page they do not have.
//
// This assertion only exists at this level. A handler test calls the
// implementation directly, with no middleware in the chain, so it would pass no
// matter what the route is gated on.
func TestMaintmodeAPIRBAC_GuestApprovalsForbidden(t *testing.T) {
	assertApprovalsForbidden(t, entity.RoleGuest)
}

func TestMaintmodeAPIRBAC_EditorApprovalsForbidden(t *testing.T) {
	assertApprovalsForbidden(t, entity.RoleEditor)
}

func assertApprovalsForbidden(t *testing.T, role entity.Role) {
	t.Helper()
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClientWithRoles(role)

	resp, err := apiClient.GetUiV1ApprovalsWithResponse(ctx, &maintmodeclient.GetUiV1ApprovalsParams{})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode())
}

// TestMaintmodeAPIRBAC_ReviewerApprovalsAllowed is the end-to-end proof of the
// personal queue: a reviewer sees the drafts assigned to them and never someone
// else's, oldest first.
//
// The token subject is the approver id, not just a role, because the filter is
// keyed on the caller's identity — setupMaintmodeTestClientWithRoles would act
// as the seeded admin instead.
//
// Assertions are by id rather than on total: the eligible approver is shared
// across the API suite on a shared database, so other tests' drafts legitimately
// sit in the same queue.
func TestMaintmodeAPIRBAC_ReviewerApprovalsAllowed(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	adminClient := setupMaintmodeTestClient()

	approverID := resolveEligibleApprover(ctx, t, adminClient)

	older := createTestMaintenance(ctx, t, adminClient)
	newer := createTestMaintenance(ctx, t, adminClient)

	// Approved, so it must have left the queue even though the approver matches.
	approved := createAndApproveMaintenance(ctx, t, adminClient)

	approverClient := setupMaintmodeTestClientWithToken(
		mustTestAccessTokenForUser(approverID.String(), entity.RoleReviewer),
	)

	resp, err := approverClient.GetUiV1ApprovalsWithResponse(ctx, &maintmodeclient.GetUiV1ApprovalsParams{
		Limit: lo.ToPtr(200),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	ids := lo.Map(lo.FromPtr(resp.JSON200.Maintenances),
		func(row maintmodeclient.ApprovalsmodelsApprovalRow, _ int) string {
			return lo.FromPtr(row.Id).String()
		})

	require.Contains(t, ids, older)
	require.Contains(t, ids, newer)
	require.NotContains(t, ids, approved, "an approved maintenance is no longer pending")
	require.Less(t, lo.IndexOf(ids, older), lo.IndexOf(ids, newer),
		"the older maintenance must come first")
}
