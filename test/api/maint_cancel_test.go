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

func TestMaintenancesAPI_CancelMaintenance(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupMaintmodeTestClient()

	maintenanceID := createAndApproveMaintenance(ctx, t, apiClient)

	cancelReq := maintmodeclient.PostApiV1MaintenancesIdCancelJSONRequestBody{
		Reason:  lo.ToPtr(maintmodeclient.MaintenanceCancelReasonIncident),
		Comment: lo.ToPtr("Critical incident requires immediate attention"),
	}

	resp, err := apiClient.PostApiV1MaintenancesIdCancelWithResponse(ctx, uuid.MustParse(maintenanceID), cancelReq)
	require.NoError(t, err, "Failed to cancel maintenance")
	require.Equal(t, http.StatusNoContent, resp.StatusCode(), "unexpected status: %s", resp.Body)

	getResp, err := apiClient.GetApiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err, "Failed to get canceled maintenance")
	require.Equal(t, http.StatusOK, getResp.StatusCode(), "unexpected status: %s", getResp.Body)
	require.NotNil(t, getResp.JSON200)

	require.Equal(t, string(maintmodeclient.MaintenanceStatusCancelled), lo.FromPtr(getResp.JSON200.Status))
	require.Equal(t, maintmodeclient.MaintenanceCancelReasonIncident, lo.FromPtr(getResp.JSON200.CancelReason))
	require.Equal(t, "Critical incident requires immediate attention", lo.FromPtr(getResp.JSON200.CancelReasonComment))
}
