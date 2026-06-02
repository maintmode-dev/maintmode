//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	maintmodeclient "github.com/ruko1202/maintmode/test/api/client/maintmode"
)

func TestMaintenancesAPI_ApproveDraft(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()

	maintenanceID := createTestMaintenance(ctx, t, apiClient)

	getResp, err := apiClient.GetApiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err, "Failed to get maintenance before approve")
	require.Equal(t, http.StatusOK, getResp.StatusCode(), "unexpected status: %s", getResp.Body)
	require.NotNil(t, getResp.JSON200)

	revision := lo.FromPtr(getResp.JSON200.CreatedAt).UnixMicro()

	approveReq := maintmodeclient.PostApiV1MaintenancesIdApproveJSONRequestBody{
		ObservedMaintRevision: lo.ToPtr(int(revision)),
		ConflictsSnapshot:     lo.ToPtr([]maintmodeclient.ApimodelsConflict{}),
	}

	resp, err := apiClient.PostApiV1MaintenancesIdApproveWithResponse(ctx, uuid.MustParse(maintenanceID), approveReq)
	require.NoError(t, err, "Failed to approve maintenance draft")
	require.Equal(t, http.StatusNoContent, resp.StatusCode(), "unexpected status: %s", resp.Body)

	getRespAfter, err := apiClient.GetApiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err, "Failed to get approved maintenance")
	require.Equal(t, http.StatusOK, getRespAfter.StatusCode(), "unexpected status: %s", getRespAfter.Body)
	require.NotNil(t, getRespAfter.JSON200)

	require.Equal(t, string(maintmodeclient.MaintenanceStatusPlanned), lo.FromPtr(getRespAfter.JSON200.Status))
}
