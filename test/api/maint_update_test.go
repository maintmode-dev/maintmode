//go:build api

package api

import (
	"context"
	"testing"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	"github.com/ruko1202/maintmode/test/api/client/client/maintenances"
	"github.com/ruko1202/maintmode/test/api/client/models"
)

func TestMaintenancesAPI_UpdateDraft(t *testing.T) {
	ctx := context.Background()

	apiClient := setupTestClient()

	maintenanceID := createTestMaintenance(ctx, t, apiClient)

	now := xtime.UTCNow()
	plannedStart := strfmt.DateTime(now.Add(48 * time.Hour))
	plannedEnd := strfmt.DateTime(now.Add(50 * time.Hour))

	resourceID := strfmt.UUID(xuuid.New().String())

	updateReq := &models.ApimodelsUpdateDraftMaintRequest{
		Title:       "Updated Maintenance Title",
		Description: "Updated description for the maintenance",
		Impact:      models.ApimodelsMaintenanceImpactPartialOutage,
		Scope:       models.ApimodelsMaintenanceScopeResource,
		PlannedPeriod: &models.ApimodelsPeriod{
			Start: plannedStart,
			End:   plannedEnd,
		},
		Resources: []*models.ApimodelsResource{
			{
				ID:   resourceID,
				Type: models.ApimodelsResourceTypeDatabase,
			},
		},
	}

	params := maintenances.NewPostAPIV1MaintenancesIDEditParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID)).
		WithRequest(updateReq)

	resp, err := apiClient.Maintenances.PostAPIV1MaintenancesIDEdit(params)
	require.NoError(t, err, "Failed to update maintenance draft")
	require.NotNil(t, resp, "Response should not be nil")

	getParams := maintenances.NewGetAPIV1MaintenancesIDParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID))

	getResp, err := apiClient.Maintenances.GetAPIV1MaintenancesID(getParams)
	require.NoError(t, err, "Failed to get updated maintenance")
	require.NotNil(t, getResp, "Get response should not be nil")

	require.Equal(t, "Updated Maintenance Title", getResp.Payload.Title)
	require.Equal(t, "Updated description for the maintenance", getResp.Payload.Description)
}
