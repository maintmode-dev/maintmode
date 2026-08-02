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
	maintmodeclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/maintmode"
)

func TestMaintenancesAPI_ApproveDraft(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupMaintmodeTestClient()

	maintenanceID := createTestMaintenance(ctx, t, apiClient)

	getResp, err := apiClient.GetApiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err, "Failed to get maintenance before approve")
	require.Equal(t, http.StatusOK, getResp.StatusCode(), "unexpected status: %s", getResp.Body)
	require.NotNil(t, getResp.JSON200)

	revision := lo.FromPtr(getResp.JSON200.CreatedAt).UnixMicro()

	approveReq := maintmodeclient.PostApiV1MaintenancesIdApproveJSONRequestBody{
		ObservedMaintRevision: lo.ToPtr(int(revision)),
		ConflictsSnapshot:     lo.ToPtr([]maintmodeclient.ApimodelsConflict{}),
	}

	// Only the assigned approver may approve, so act as that user.
	approverID := lo.FromPtr(lo.FromPtr(getResp.JSON200.Approver).Id)
	approverClient := setupMaintmodeTestClientWithToken(
		mustTestAccessTokenForUser(approverID.String(), entity.RoleReviewer),
	)

	resp, err := approverClient.PostApiV1MaintenancesIdApproveWithResponse(ctx, uuid.MustParse(maintenanceID), approveReq)
	require.NoError(t, err, "Failed to approve maintenance draft")
	require.Equal(t, http.StatusNoContent, resp.StatusCode(), "unexpected status: %s", resp.Body)

	getRespAfter, err := apiClient.GetApiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err, "Failed to get approved maintenance")
	require.Equal(t, http.StatusOK, getRespAfter.StatusCode(), "unexpected status: %s", getRespAfter.Body)
	require.NotNil(t, getRespAfter.JSON200)

	require.Equal(t, string(maintmodeclient.MaintenanceStatusPlanned), lo.FromPtr(getRespAfter.JSON200.Status))
}

// An admin may approve a maintenance assigned to someone else. This is the
// escape hatch for a draft whose assigned approver can no longer act (left,
// demoted, blocked) — without it the maintenance would be stuck in draft with
// cancel as the only, irreversible way out.
func TestMaintenancesAPI_ApproveDraft_AdminOnSomeoneElsesMaintenance(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupMaintmodeTestClient()

	maintenanceID := createTestMaintenance(ctx, t, apiClient)

	getResp, err := apiClient.GetApiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err, "Failed to get maintenance before approve")
	require.Equal(t, http.StatusOK, getResp.StatusCode(), "unexpected status: %s", getResp.Body)
	require.NotNil(t, getResp.JSON200)

	approverID := lo.FromPtr(lo.FromPtr(getResp.JSON200.Approver).Id)

	// An admin who is NOT the assigned approver.
	adminClient, adminID := provisionUser(ctx, t, entity.RoleAdmin)
	require.NotEqual(t, approverID, adminID, "admin must not be the assigned approver")

	approveReq := maintmodeclient.PostApiV1MaintenancesIdApproveJSONRequestBody{
		ObservedMaintRevision: lo.ToPtr(int(lo.FromPtr(getResp.JSON200.CreatedAt).UnixMicro())),
		ConflictsSnapshot:     lo.ToPtr([]maintmodeclient.ApimodelsConflict{}),
	}

	resp, err := adminClient.PostApiV1MaintenancesIdApproveWithResponse(ctx, uuid.MustParse(maintenanceID), approveReq)
	require.NoError(t, err, "Failed to approve maintenance as admin")
	require.Equal(t, http.StatusNoContent, resp.StatusCode(), "unexpected status: %s", resp.Body)

	getRespAfter, err := apiClient.GetApiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err, "Failed to get approved maintenance")
	require.Equal(t, http.StatusOK, getRespAfter.StatusCode(), "unexpected status: %s", getRespAfter.Body)
	require.Equal(t, string(maintmodeclient.MaintenanceStatusPlanned), lo.FromPtr(getRespAfter.JSON200.Status))

	// The assignment itself is untouched — the admin approved on their behalf,
	// they did not become the approver.
	require.Equal(t, approverID, lo.FromPtr(lo.FromPtr(getRespAfter.JSON200.Approver).Id))
}

// A reviewer holds maintenance.approve, so RBAC lets the request through; only
// the assignment guard stops them. Unlike an admin, they stay bound to it.
func TestMaintenancesAPI_ApproveDraft_ReviewerOnSomeoneElsesMaintenance(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupMaintmodeTestClient()

	maintenanceID := createTestMaintenance(ctx, t, apiClient)

	getResp, err := apiClient.GetApiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err, "Failed to get maintenance before approve")
	require.Equal(t, http.StatusOK, getResp.StatusCode(), "unexpected status: %s", getResp.Body)
	require.NotNil(t, getResp.JSON200)

	reviewerClient, reviewerID := provisionUser(ctx, t, entity.RoleReviewer)
	require.NotEqual(t,
		lo.FromPtr(lo.FromPtr(getResp.JSON200.Approver).Id), reviewerID,
		"reviewer must not be the assigned approver",
	)

	approveReq := maintmodeclient.PostApiV1MaintenancesIdApproveJSONRequestBody{
		ObservedMaintRevision: lo.ToPtr(int(lo.FromPtr(getResp.JSON200.CreatedAt).UnixMicro())),
		ConflictsSnapshot:     lo.ToPtr([]maintmodeclient.ApimodelsConflict{}),
	}

	resp, err := reviewerClient.PostApiV1MaintenancesIdApproveWithResponse(ctx, uuid.MustParse(maintenanceID), approveReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode(), "unexpected status: %s", resp.Body)

	// Still a draft: the rejected approve changed nothing.
	getRespAfter, err := apiClient.GetApiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err)
	require.Equal(t, string(maintmodeclient.MaintenanceStatusDraft), lo.FromPtr(getRespAfter.JSON200.Status))
}
