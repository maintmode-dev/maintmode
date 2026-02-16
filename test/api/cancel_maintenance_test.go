//go:build api

package api_test

import (
	"context"
	"testing"

	"github.com/go-openapi/strfmt"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/test/api/client/client/maintenances"
	"github.com/ruko1202/maintmode/test/api/client/models"
)

func TestMaintenancesAPI_CancelMaintenance(t *testing.T) {
	ctx := context.Background()

	apiClient := setupTestClient()

	maintenanceID := createAndApproveMaintenance(ctx, t, apiClient)

	cancelReq := &models.ApimodelsCancelMaintRequest{
		Reason:  models.ApimodelsMaintenanceCancelReasonIncident,
		Comment: "Critical incident requires immediate attention",
	}

	params := maintenances.NewPostAPIV1MaintenancesIDCancelParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID)).
		WithRequest(cancelReq)

	resp, err := apiClient.Maintenances.PostAPIV1MaintenancesIDCancel(params)
	require.NoError(t, err, "Failed to cancel maintenance")
	require.NotNil(t, resp, "Response should not be nil")

	getParams := maintenances.NewGetAPIV1MaintenancesIDParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID))

	getResp, err := apiClient.Maintenances.GetAPIV1MaintenancesID(getParams)
	require.NoError(t, err, "Failed to get canceled maintenance")
	require.NotNil(t, getResp, "Get response should not be nil")

	require.Equal(t, string(models.UimodelsMaintenanceStatusCanceled), getResp.Payload.Status)
	require.Equal(t, models.ApimodelsMaintenanceCancelReasonIncident, getResp.Payload.CancelReason)
	require.Equal(t, "Critical incident requires immediate attention", getResp.Payload.CancelReasonComment)
}
