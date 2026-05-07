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

func TestMaintenancesAPI_StepLifecycle(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()
	maintenanceID := createAndStartMaintenance(ctx, t, apiClient)

	getParams := maintenances.NewGetAPIV1MaintenancesIDParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID))

	getResp, err := apiClient.Maintenances.GetAPIV1MaintenancesID(getParams, nil)
	require.NoError(t, err)
	require.NotNil(t, getResp)
	require.NotEmpty(t, getResp.Payload.Steps)

	stepID := getResp.Payload.Steps[0].ID

	startParams := maintenances.NewPostAPIV1MaintenancesIDStepsStepIDStartParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID)).
		WithStepID(stepID)

	_, err = apiClient.Maintenances.PostAPIV1MaintenancesIDStepsStepIDStart(startParams, nil)
	require.NoError(t, err)

	getResp, err = apiClient.Maintenances.GetAPIV1MaintenancesID(getParams, nil)
	require.NoError(t, err)
	require.Equal(t, models.ApimodelsMaintenanceStepStatusStarted, getResp.Payload.Steps[0].Status)

	cancelParams := maintenances.NewPostAPIV1MaintenancesIDStepsStepIDCancelParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID)).
		WithStepID(stepID)

	_, err = apiClient.Maintenances.PostAPIV1MaintenancesIDStepsStepIDCancel(cancelParams, nil)
	require.NoError(t, err)

	getResp, err = apiClient.Maintenances.GetAPIV1MaintenancesID(getParams, nil)
	require.NoError(t, err)
	require.Equal(t, models.ApimodelsMaintenanceStepStatusCanceled, getResp.Payload.Steps[0].Status)
}
