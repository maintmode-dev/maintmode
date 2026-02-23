//go:build api

package api

import (
	"context"
	"testing"

	"github.com/go-openapi/strfmt"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/test/api/client/client/maintenances"
	"github.com/ruko1202/maintmode/test/api/client/models"
)

func TestMaintenancesAPI_CompleteMaintenance(t *testing.T) {
	ctx := context.Background()

	apiClient := setupTestClient()

	maintenanceID := createAndStartMaintenance(ctx, t, apiClient)

	params := maintenances.NewPostAPIV1MaintenancesIDCompleteParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID))

	resp, err := apiClient.Maintenances.PostAPIV1MaintenancesIDComplete(params)
	require.NoError(t, err, "Failed to complete maintenance")
	require.NotNil(t, resp, "Response should not be nil")

	getParams := maintenances.NewGetAPIV1MaintenancesIDParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID))

	getResp, err := apiClient.Maintenances.GetAPIV1MaintenancesID(getParams)
	require.NoError(t, err, "Failed to get completed maintenance")
	require.NotNil(t, getResp, "Get response should not be nil")

	require.Equal(t, string(models.UimodelsMaintenanceStatusCompleted), getResp.Payload.Status)
	require.NotNil(t, getResp.Payload.ActualPeriod)
	require.False(t, getResp.Payload.ActualPeriod.End.IsZero())
}
