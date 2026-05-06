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

func TestMaintenancesAPI_StartMaintenance(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()

	maintenanceID := createAndApproveMaintenance(ctx, t, apiClient)

	params := maintenances.NewPostAPIV1MaintenancesIDStartParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID))

	resp, err := apiClient.Maintenances.PostAPIV1MaintenancesIDStart(params, nil)
	require.NoError(t, err, "Failed to start maintenance")
	require.NotNil(t, resp, "Response should not be nil")

	getParams := maintenances.NewGetAPIV1MaintenancesIDParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID))

	getResp, err := apiClient.Maintenances.GetAPIV1MaintenancesID(getParams, nil)
	require.NoError(t, err, "Failed to get started maintenance")
	require.NotNil(t, getResp, "Get response should not be nil")

	require.Equal(t, string(models.UimodelsMaintenanceStatusInProgress), getResp.Payload.Status)
	require.NotNil(t, getResp.Payload.ActualPeriod)
	require.False(t, getResp.Payload.ActualPeriod.Start.IsZero())
}
