//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"

	maintmodeclient "github.com/ruko1202/maintmode/test/api/client/maintmode"
)

func TestUIAPI_GetMaintenanceView(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()

	maintenanceID := createTestMaintenance(ctx, t, apiClient)

	resp, err := apiClient.GetUiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err, "Failed to get maintenance view")
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	payload := resp.JSON200
	require.NotNil(t, payload.Maintenance, "Maintenance should not be nil")
	require.NotNil(t, payload.Actions, "Actions should not be nil")
	require.NotNil(t, payload.Conflicts, "Conflicts should not be nil")

	maint := payload.Maintenance
	require.Equal(t, maintenanceID, lo.FromPtr(maint.Id).String())
	require.Equal(t, "Test Maintenance", lo.FromPtr(maint.Title))
	require.Equal(t, maintmodeclient.MaintenanceStatusDraft, lo.FromPtr(maint.Status))

	actions := payload.Actions
	require.True(t, lo.FromPtr(actions.CanEdit), "Should be able to edit draft")
	require.True(t, lo.FromPtr(actions.CanApprove), "Should be able to approve draft")
	require.False(t, lo.FromPtr(actions.CanStart), "Should not be able to start draft")
	require.False(t, lo.FromPtr(actions.CanComplete), "Should not be able to finish draft")
}

func TestUIAPI_GetMaintenanceView_Planned(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()

	maintenanceID := createAndApproveMaintenance(ctx, t, apiClient)

	resp, err := apiClient.GetUiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err, "Failed to get maintenance view for planned maintenance")
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	payload := resp.JSON200
	maint := payload.Maintenance
	require.Equal(t, maintmodeclient.MaintenanceStatusPlanned, lo.FromPtr(maint.Status))

	actions := payload.Actions
	require.False(t, lo.FromPtr(actions.CanEdit), "Should not be able to edit planned maintenance")
	require.False(t, lo.FromPtr(actions.CanApprove), "Should not be able to approve planned maintenance")
	require.True(t, lo.FromPtr(actions.CanStart), "Should be able to start planned maintenance")
	require.True(t, lo.FromPtr(actions.CanCancel), "Should be able to cancel planned maintenance")
}

func TestUIAPI_GetMaintenanceView_InProgress(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()

	maintenanceID := createAndStartMaintenance(ctx, t, apiClient)
	completeMaintenanceSteps(ctx, t, apiClient, maintenanceID)

	resp, err := apiClient.GetUiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err, "Failed to get maintenance view for in-progress maintenance")
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	payload := resp.JSON200
	maint := payload.Maintenance
	require.Equal(t, maintmodeclient.MaintenanceStatusInProgress, lo.FromPtr(maint.Status))
	require.False(t, lo.FromPtr(maint.ActualTimeStart).IsZero(), "Actual start time should be set")

	actions := payload.Actions
	require.False(t, lo.FromPtr(actions.CanEdit), "Should not be able to edit in-progress maintenance")
	require.False(t, lo.FromPtr(actions.CanApprove), "Should not be able to approve in-progress maintenance")
	require.False(t, lo.FromPtr(actions.CanStart), "Should not be able to start in-progress maintenance")
	require.True(t, lo.FromPtr(actions.CanComplete), "Should be able to finish in-progress maintenance")
	require.True(t, lo.FromPtr(actions.CanCancel), "Should be able to cancel in-progress maintenance")
}

func TestUIAPI_GetMaintenanceView_WithConflicts(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()

	resourceID := lo.FromPtr(creatResource(ctx, t, apiClient).Id)

	maintenanceID1 := createMaintenanceWithResource(ctx, t, apiClient, resourceID)
	approveMaintenance(ctx, t, apiClient, maintenanceID1)

	maintenanceID2 := createMaintenanceWithResource(ctx, t, apiClient, resourceID)

	resp, err := apiClient.GetUiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID2))
	require.NoError(t, err, "Failed to get maintenance view with conflicts")
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	payload := resp.JSON200
	require.NotNil(t, payload.Conflicts, "Conflicts should not be nil")
	require.GreaterOrEqual(t, len(lo.FromPtr(payload.Conflicts)), 1, "Should have at least 1 conflict")

	conflict := lo.FromPtr(payload.Conflicts)[0]
	require.Equal(t, maintenanceID1, lo.FromPtr(conflict.MaintenanceId).String(), "Conflict should reference the first maintenance")
	require.NotNil(t, conflict.Resources, "Conflict resources should not be nil")
}

func TestUIAPI_GetMaintenanceView_NonExistent(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()

	nonExistentID := xuuid.New()

	resp, err := apiClient.GetUiV1MaintenancesIdWithResponse(ctx, nonExistentID)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode(), "Should return not found for non-existent maintenance")
	require.NotNil(t, resp.JSON404, "Error payload should not be nil")
}
