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

	"github.com/ruko1202/maintmode/internal/utils/xtime"
	maintmodeclient "github.com/ruko1202/maintmode/test/api/client/maintmode"
)

func TestMaintenancesAPI_UpdateDraft(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()

	maintenanceID := createTestMaintenance(ctx, t, apiClient)

	now := xtime.UTCNow()
	plannedStart := now.Add(48 * time.Hour).Truncate(time.Second)

	resource := creatResource(ctx, t, apiClient)

	updateReq := maintmodeclient.PostApiV1MaintenancesIdEditJSONRequestBody{
		Title:        lo.ToPtr("Updated Maintenance Title"),
		Description:  lo.ToPtr("Updated description for the maintenance"),
		Impact:       lo.ToPtr(maintmodeclient.MaintenanceImpactPartial),
		Scope:        lo.ToPtr(maintmodeclient.MaintenanceScopeResources),
		PlannedStart: lo.ToPtr(plannedStart),
		Resources: lo.ToPtr([]maintmodeclient.ApimodelsResourceRef{
			{Id: lo.ToPtr(uuid.MustParse(lo.FromPtr(resource.Id)))},
		}),
		Steps:         lo.ToPtr(testMaintenanceSteps()),
		NotifyTargets: testNotifyTargets(ctx, t, apiClient),
	}

	resp, err := apiClient.PostApiV1MaintenancesIdEditWithResponse(ctx, uuid.MustParse(maintenanceID), updateReq)
	require.NoError(t, err, "Failed to update maintenance draft")
	require.Equal(t, http.StatusNoContent, resp.StatusCode(), "unexpected status: %s", resp.Body)

	getResp, err := apiClient.GetApiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err, "Failed to get updated maintenance")
	require.Equal(t, http.StatusOK, getResp.StatusCode(), "unexpected status: %s", getResp.Body)
	require.NotNil(t, getResp.JSON200)

	require.Equal(t, "Updated Maintenance Title", lo.FromPtr(getResp.JSON200.Title))
	require.Equal(t, "Updated description for the maintenance", lo.FromPtr(getResp.JSON200.Description))
	require.NotNil(t, getResp.JSON200.PlannedPeriod)
	require.Equal(t, plannedStart, lo.FromPtr(getResp.JSON200.PlannedPeriod.Start))
	require.Equal(t, plannedStart.Add(testMaintenanceDuration), lo.FromPtr(getResp.JSON200.PlannedPeriod.End))
}
