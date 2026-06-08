//go:build api

package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	maintmodeclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/maintmode"
)

func TestUIAPI_GetCalendarView(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupMaintmodeTestClient()

	createTestMaintenance(ctx, t, apiClient)
	createAndApproveMaintenance(ctx, t, apiClient)

	fromDate := openapi_types.Date{Time: time.Now().AddDate(0, 0, -7)}
	toDate := openapi_types.Date{Time: time.Now().AddDate(0, 0, 30)}

	resp, err := apiClient.GetUiV1CalendarWithResponse(ctx, &maintmodeclient.GetUiV1CalendarParams{
		From: fromDate,
		To:   toDate,
	})
	require.NoError(t, err, "Failed to get calendar view")
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	payload := resp.JSON200
	require.NotNil(t, payload.Events, "Events should not be nil")
	require.NotNil(t, payload.Meta, "Meta should not be nil")
	require.GreaterOrEqual(t, len(lo.FromPtr(payload.Events)), 2, "Should have at least 2 events")
}

func TestUIAPI_GetCalendarView_WithStatusFilter(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupMaintmodeTestClient()

	createTestMaintenance(ctx, t, apiClient)
	createAndApproveMaintenance(ctx, t, apiClient)

	fromDate := openapi_types.Date{Time: time.Now().AddDate(0, 0, -7)}
	toDate := openapi_types.Date{Time: time.Now().AddDate(0, 0, 30)}
	statuses := []maintmodeclient.GetUiV1CalendarParamsStatuses{maintmodeclient.Draft}

	resp, err := apiClient.GetUiV1CalendarWithResponse(ctx, &maintmodeclient.GetUiV1CalendarParams{
		From:     fromDate,
		To:       toDate,
		Statuses: lo.ToPtr(statuses),
	})
	require.NoError(t, err, "Failed to get calendar view with status filter")
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	payload := resp.JSON200
	require.NotNil(t, payload.Events, "Events should not be nil")

	for _, event := range lo.FromPtr(payload.Events) {
		require.Equal(t, maintmodeclient.MaintenanceStatusDraft, lo.FromPtr(event.Status), "Event should have draft status")
	}
}

func TestUIAPI_GetCalendarView_WithResourceFilter(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupMaintmodeTestClient()

	resourceID := lo.FromPtr(creatResource(ctx, t, apiClient).Id)
	maintenanceID := createMaintenanceWithResource(ctx, t, apiClient, resourceID)
	require.NotEmpty(t, maintenanceID, "Should create maintenance with specific resource")

	fromDate := openapi_types.Date{Time: time.Now().AddDate(0, 0, -7)}
	toDate := openapi_types.Date{Time: time.Now().AddDate(0, 0, 30)}
	resourceIDs := []string{resourceID}

	resp, err := apiClient.GetUiV1CalendarWithResponse(ctx, &maintmodeclient.GetUiV1CalendarParams{
		From:        fromDate,
		To:          toDate,
		ResourceIds: lo.ToPtr(resourceIDs),
	})
	require.NoError(t, err, "Failed to get calendar view with resource filter")
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	payload := resp.JSON200
	require.NotNil(t, payload.Events, "Events should not be nil")
	require.GreaterOrEqual(t, len(lo.FromPtr(payload.Events)), 1, "Should have at least 1 event")
}

func TestUIAPI_GetCalendarView_WithChannelFilter(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupMaintmodeTestClient()

	// Dedicated channel so the filter matches only the maintenance below.
	channelID := createNotifyChannel(ctx, t, apiClient)
	maintenanceID := createMaintenanceWithChannel(ctx, t, apiClient, channelID)
	require.NotEmpty(t, maintenanceID, "Should create maintenance with specific channel")

	// A second maintenance on a different channel must be filtered out.
	createTestMaintenance(ctx, t, apiClient)

	fromDate := openapi_types.Date{Time: time.Now().AddDate(0, 0, -7)}
	toDate := openapi_types.Date{Time: time.Now().AddDate(0, 0, 30)}
	channelIDs := []string{channelID}

	resp, err := apiClient.GetUiV1CalendarWithResponse(ctx, &maintmodeclient.GetUiV1CalendarParams{
		From:       fromDate,
		To:         toDate,
		ChannelIds: lo.ToPtr(channelIDs),
	})
	require.NoError(t, err, "Failed to get calendar view with channel filter")
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	payload := resp.JSON200
	require.NotNil(t, payload.Events, "Events should not be nil")
	require.Len(t, lo.FromPtr(payload.Events), 1, "Only the maintenance on the filtered channel should be returned")
	require.Equal(t, maintenanceID, lo.FromPtr(payload.Events)[0].Id.String())
}
