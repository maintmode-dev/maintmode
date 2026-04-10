//go:build api

package api

import (
	"context"
	"testing"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/test/api/client/client/maintenances"
	"github.com/ruko1202/maintmode/test/api/client/models"
)

func TestMaintenancesAPI_ApproveDraft(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()

	maintenanceID := createTestMaintenance(ctx, t, apiClient)

	getParams := maintenances.NewGetAPIV1MaintenancesIDParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID))

	getResp, err := apiClient.Maintenances.GetAPIV1MaintenancesID(getParams)
	require.NoError(t, err, "Failed to get maintenance before approve")
	require.NotNil(t, getResp, "Get response should not be nil")

	revision := time.Time(getResp.Payload.CreatedAt).UnixMicro()

	approveReq := &models.ApimodelsApproveDraftMaintRequest{
		ObservedMaintRevision: revision,
		ConflictsSnapshot:     []*models.ApimodelsConflict{},
	}

	params := maintenances.NewPostAPIV1MaintenancesIDApproveParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID)).
		WithRequest(approveReq)

	resp, err := apiClient.Maintenances.PostAPIV1MaintenancesIDApprove(params)
	require.NoError(t, err, "Failed to approve maintenance draft")
	require.NotNil(t, resp, "Response should not be nil")

	getParamsAfter := maintenances.NewGetAPIV1MaintenancesIDParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID))

	getRespAfter, err := apiClient.Maintenances.GetAPIV1MaintenancesID(getParamsAfter)
	require.NoError(t, err, "Failed to get approved maintenance")
	require.NotNil(t, getRespAfter, "Get response should not be nil")

	require.Equal(t, string(models.UimodelsMaintenanceStatusPlanned), getRespAfter.Payload.Status)
}
