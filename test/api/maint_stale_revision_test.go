//go:build api

package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	maintmodeclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/maintmode"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestMaintenancesAPI_ApproveRejectsStaleRevision(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupMaintmodeTestClient()
	maintenanceID := createTestMaintenance(ctx, t, apiClient)

	getResp, err := apiClient.GetApiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, getResp.StatusCode(), "unexpected status: %s", getResp.Body)
	require.NotNil(t, getResp.JSON200)

	staleRevision := lo.FromPtr(getResp.JSON200.CreatedAt).UnixMicro()

	resource := creatResource(ctx, t, apiClient)
	updateReq := maintmodeclient.PostApiV1MaintenancesIdEditJSONRequestBody{
		Title:        lo.ToPtr("Revision changed maintenance"),
		Description:  lo.ToPtr("Update before approval must change the revision"),
		Impact:       lo.ToPtr(maintmodeclient.MaintenanceImpactPartial),
		Scope:        lo.ToPtr(maintmodeclient.MaintenanceScopeResources),
		PlannedStart: lo.ToPtr(xtime.UTCNow().Add(72 * time.Hour).Truncate(time.Second)),
		Resources: lo.ToPtr([]maintmodeclient.ApimodelsResourceRef{
			{Id: lo.ToPtr(uuid.MustParse(lo.FromPtr(resource.Id)))},
		}),
		Steps:         lo.ToPtr(testMaintenanceSteps()),
		NotifyTargets: testNotifyTargets(ctx, t, apiClient),
	}

	editResp, err := apiClient.PostApiV1MaintenancesIdEditWithResponse(ctx, uuid.MustParse(maintenanceID), updateReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, editResp.StatusCode(), "unexpected status: %s", editResp.Body)

	approveReq := maintmodeclient.PostApiV1MaintenancesIdApproveJSONRequestBody{
		ObservedMaintRevision: lo.ToPtr(int(staleRevision)),
		ConflictsSnapshot:     lo.ToPtr([]maintmodeclient.ApimodelsConflict{}),
	}

	// Act as the assigned approver so the request reaches the revision check
	// rather than being rejected by the approver-mismatch guard.
	approverID := lo.FromPtr(lo.FromPtr(getResp.JSON200.Approver).Id)
	approverClient := setupMaintmodeTestClientWithToken(
		mustTestAccessTokenForUser(approverID.String(), entity.RoleReviewer),
	)

	approveResp, err := approverClient.PostApiV1MaintenancesIdApproveWithResponse(ctx, uuid.MustParse(maintenanceID), approveReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, approveResp.StatusCode(), "unexpected status: %s", approveResp.Body)
}
