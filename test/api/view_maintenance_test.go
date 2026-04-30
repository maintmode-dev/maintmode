//go:build api

package api

import (
	"context"
	"testing"

	"github.com/go-openapi/strfmt"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"

	"github.com/ruko1202/maintmode/test/api/client/client/ui"
	"github.com/ruko1202/maintmode/test/api/client/models"
)

func TestUIAPI_GetMaintenanceView(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()

	maintenanceID := createTestMaintenance(ctx, t, apiClient)

	params := ui.NewGetUIV1MaintenancesIDParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID))

	resp, err := apiClient.UI.GetUIV1MaintenancesID(params)
	require.NoError(t, err, "Failed to get maintenance view")
	require.NotNil(t, resp, "Response should not be nil")

	payload := resp.Payload
	require.NotNil(t, payload.Maintenance, "Maintenance should not be nil")
	require.NotNil(t, payload.Actions, "Actions should not be nil")
	require.NotNil(t, payload.Conflicts, "Conflicts should not be nil")

	maint := payload.Maintenance
	require.Equal(t, maintenanceID, maint.ID.String())
	require.Equal(t, "Test Maintenance", maint.Title)
	require.Equal(t, models.UimodelsMaintenanceStatusDraft, maint.Status)

	actions := payload.Actions
	require.True(t, actions.CanEdit, "Should be able to edit draft")
	require.True(t, actions.CanApprove, "Should be able to approve draft")
	require.False(t, actions.CanStart, "Should not be able to start draft")
	require.False(t, actions.CanFinish, "Should not be able to finish draft")
}

func TestUIAPI_GetMaintenanceView_Planned(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()

	maintenanceID := createAndApproveMaintenance(ctx, t, apiClient)

	params := ui.NewGetUIV1MaintenancesIDParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID))

	resp, err := apiClient.UI.GetUIV1MaintenancesID(params)
	require.NoError(t, err, "Failed to get maintenance view for planned maintenance")
	require.NotNil(t, resp, "Response should not be nil")

	payload := resp.Payload
	maint := payload.Maintenance
	require.Equal(t, models.UimodelsMaintenanceStatusPlanned, maint.Status)

	actions := payload.Actions
	require.False(t, actions.CanEdit, "Should not be able to edit planned maintenance")
	require.False(t, actions.CanApprove, "Should not be able to approve planned maintenance")
	require.True(t, actions.CanStart, "Should be able to start planned maintenance")
	require.True(t, actions.CanCancel, "Should be able to cancel planned maintenance")
}

func TestUIAPI_GetMaintenanceView_InProgress(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()

	maintenanceID := createAndStartMaintenance(ctx, t, apiClient)
	completeMaintenanceSteps(ctx, t, apiClient, maintenanceID)

	params := ui.NewGetUIV1MaintenancesIDParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID))

	resp, err := apiClient.UI.GetUIV1MaintenancesID(params)
	require.NoError(t, err, "Failed to get maintenance view for in-progress maintenance")
	require.NotNil(t, resp, "Response should not be nil")

	payload := resp.Payload
	maint := payload.Maintenance
	require.Equal(t, models.UimodelsMaintenanceStatusInProgress, maint.Status)
	require.False(t, maint.ActualTimeStart.IsZero(), "Actual start time should be set")

	actions := payload.Actions
	require.False(t, actions.CanEdit, "Should not be able to edit in-progress maintenance")
	require.False(t, actions.CanApprove, "Should not be able to approve in-progress maintenance")
	require.False(t, actions.CanStart, "Should not be able to start in-progress maintenance")
	require.True(t, actions.CanFinish, "Should be able to finish in-progress maintenance")
	require.True(t, actions.CanCancel, "Should be able to cancel in-progress maintenance")
}

func TestUIAPI_GetMaintenanceView_WithConflicts(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()

	resourceID := xuuid.New().String()

	maintenanceID1 := createMaintenanceWithResource(ctx, t, apiClient, resourceID)
	approveMaintenance(ctx, t, apiClient, maintenanceID1)

	maintenanceID2 := createMaintenanceWithResource(ctx, t, apiClient, resourceID)

	params := ui.NewGetUIV1MaintenancesIDParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID2))

	resp, err := apiClient.UI.GetUIV1MaintenancesID(params)
	require.NoError(t, err, "Failed to get maintenance view with conflicts")
	require.NotNil(t, resp, "Response should not be nil")

	payload := resp.Payload
	require.NotNil(t, payload.Conflicts, "Conflicts should not be nil")
	require.GreaterOrEqual(t, len(payload.Conflicts), 1, "Should have at least 1 conflict")

	conflict := payload.Conflicts[0]
	require.Equal(t, maintenanceID1, conflict.MaintenanceID.String(), "Conflict should reference the first maintenance")
	require.NotNil(t, conflict.Resources, "Conflict resources should not be nil")
}

func TestUIAPI_GetMaintenanceView_NonExistent(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()

	nonExistentID := strfmt.UUID(xuuid.New().String())
	params := ui.NewGetUIV1MaintenancesIDParams().
		WithContext(ctx).
		WithID(nonExistentID)

	_, err := apiClient.UI.GetUIV1MaintenancesID(params)
	require.Error(t, err, "Should return error for non-existent maintenance")

	var apiErr *ui.GetUIV1MaintenancesIDNotFound
	require.ErrorAs(t, err, &apiErr, "Expected GetUIV1MaintenancesIDNotFound error")
	require.NotNil(t, apiErr.Payload, "Error payload should not be nil")
}
