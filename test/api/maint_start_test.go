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

func TestMaintenancesAPI_StartMaintenance(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupMaintmodeTestClient()

	maintenanceID := createAndApproveMaintenance(ctx, t, apiClient)

	resp, err := apiClient.PostApiV1MaintenancesIdStartWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err, "Failed to start maintenance")
	require.Equal(t, http.StatusNoContent, resp.StatusCode(), "unexpected status: %s", resp.Body)

	getResp, err := apiClient.GetApiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err, "Failed to get started maintenance")
	require.Equal(t, http.StatusOK, getResp.StatusCode(), "unexpected status: %s", getResp.Body)
	require.NotNil(t, getResp.JSON200)

	require.Equal(t, string(maintmodeclient.MaintenanceStatusInProgress), lo.FromPtr(getResp.JSON200.Status))
	require.NotNil(t, getResp.JSON200.ActualPeriod)
	require.False(t, lo.FromPtr(getResp.JSON200.ActualPeriod.Start).IsZero())
}
