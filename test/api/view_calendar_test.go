//go:build api

package api

import (
	"context"
	"testing"
	"time"

	"github.com/go-openapi/strfmt"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/test/api/client/client/ui"
	"github.com/ruko1202/maintmode/test/api/client/models"
)

func TestUIAPI_GetCalendarView(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()

	createTestMaintenance(ctx, t, apiClient)
	createAndApproveMaintenance(ctx, t, apiClient)

	fromDate := strfmt.Date(time.Now().AddDate(0, 0, -7))
	toDate := strfmt.Date(time.Now().AddDate(0, 0, 30))

	params := ui.NewGetUIV1CalendarParams().
		WithContext(ctx).
		WithFrom(fromDate).
		WithTo(toDate)

	resp, err := apiClient.UI.GetUIV1Calendar(params, nil)
	require.NoError(t, err, "Failed to get calendar view")
	require.NotNil(t, resp, "Response should not be nil")

	payload := resp.Payload
	require.NotNil(t, payload.Events, "Events should not be nil")
	require.NotNil(t, payload.Meta, "Meta should not be nil")
	require.GreaterOrEqual(t, len(payload.Events), 2, "Should have at least 2 events")
}

func TestUIAPI_GetCalendarView_WithStatusFilter(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()

	createTestMaintenance(ctx, t, apiClient)
	createAndApproveMaintenance(ctx, t, apiClient)

	fromDate := strfmt.Date(time.Now().AddDate(0, 0, -7))
	toDate := strfmt.Date(time.Now().AddDate(0, 0, 30))
	statuses := []string{string(models.UimodelsMaintenanceStatusDraft)}

	params := ui.NewGetUIV1CalendarParams().
		WithContext(ctx).
		WithFrom(fromDate).
		WithTo(toDate).
		WithStatuses(statuses)

	resp, err := apiClient.UI.GetUIV1Calendar(params, nil)
	require.NoError(t, err, "Failed to get calendar view with status filter")
	require.NotNil(t, resp, "Response should not be nil")

	payload := resp.Payload
	require.NotNil(t, payload.Events, "Events should not be nil")

	for _, event := range payload.Events {
		require.Equal(t, models.UimodelsMaintenanceStatusDraft, event.Status, "Event should have draft status")
	}
}

func TestUIAPI_GetCalendarView_WithResourceFilter(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()

	resourceID := creatResource(ctx, t, apiClient).ID
	maintenanceID := createMaintenanceWithResource(ctx, t, apiClient, resourceID)
	require.NotEmpty(t, maintenanceID, "Should create maintenance with specific resource")

	fromDate := strfmt.Date(time.Now().AddDate(0, 0, -7))
	toDate := strfmt.Date(time.Now().AddDate(0, 0, 30))
	resourceIDs := []string{resourceID}

	params := ui.NewGetUIV1CalendarParams().
		WithContext(ctx).
		WithFrom(fromDate).
		WithTo(toDate).
		WithResourceIds(resourceIDs)

	resp, err := apiClient.UI.GetUIV1Calendar(params, nil)
	require.NoError(t, err, "Failed to get calendar view with resource filter")
	require.NotNil(t, resp, "Response should not be nil")

	payload := resp.Payload
	require.NotNil(t, payload.Events, "Events should not be nil")
	require.GreaterOrEqual(t, len(payload.Events), 1, "Should have at least 1 event")
}
