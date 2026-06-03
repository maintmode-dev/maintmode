//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	maintmodeclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/maintmode"
)

func TestMaintenancesAPI_StepLifecycle(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()
	maintenanceID := createAndStartMaintenance(ctx, t, apiClient)

	getResp, err := apiClient.GetApiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, getResp.StatusCode(), "unexpected status: %s", getResp.Body)
	require.NotNil(t, getResp.JSON200)
	require.NotEmpty(t, lo.FromPtr(getResp.JSON200.Steps))

	stepID := lo.FromPtr(lo.FromPtr(getResp.JSON200.Steps)[0].Id)

	startResp, err := apiClient.PostApiV1MaintenancesIdStepsStepIdStartWithResponse(ctx, uuid.MustParse(maintenanceID), stepID)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, startResp.StatusCode(), "unexpected status: %s", startResp.Body)

	getResp, err = apiClient.GetApiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, getResp.StatusCode(), "unexpected status: %s", getResp.Body)
	require.Equal(t, maintmodeclient.MaintenanceStepStatusStarted, lo.FromPtr(lo.FromPtr(getResp.JSON200.Steps)[0].Status))

	cancelResp, err := apiClient.PostApiV1MaintenancesIdStepsStepIdCancelWithResponse(ctx, uuid.MustParse(maintenanceID), stepID)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, cancelResp.StatusCode(), "unexpected status: %s", cancelResp.Body)

	getResp, err = apiClient.GetApiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, getResp.StatusCode(), "unexpected status: %s", getResp.Body)
	require.Equal(t, maintmodeclient.MaintenanceStepStatusCanceled, lo.FromPtr(lo.FromPtr(getResp.JSON200.Steps)[0].Status))
}
