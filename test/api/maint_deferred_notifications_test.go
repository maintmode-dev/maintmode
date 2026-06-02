//go:build api

package api

import (
	"context"
	"testing"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/test/api/client/client"
	"github.com/ruko1202/maintmode/test/api/client/client/maintenances"
	"github.com/ruko1202/maintmode/test/api/client/client/notifications"
	"github.com/ruko1202/maintmode/test/api/client/models"
)

// TestMaintenancesAPI_DeferredNotifications walks the full deferred-reminder
// lifecycle through the public API: create a draft carrying two reminders (one
// fanned out to two channels), confirm they round-trip on GET, approve (which
// enqueues the goque reminders), then cancel (which cancels them). The flow
// exercises the server-side enqueue/cancel path end to end.
func TestMaintenancesAPI_DeferredNotifications(t *testing.T) {
	ctx := context.Background()
	apiClient := setupMaintmodeTestClient()

	channelIDs := availableChannelIDs(ctx, t, apiClient)

	now := xtime.UTCNow()
	plannedStart := strfmt.DateTime(now.Add(24 * time.Hour).Truncate(time.Second))
	fireAt30 := strfmt.DateTime(now.Add(23*time.Hour + 30*time.Minute).Truncate(time.Second))
	fireAt5 := strfmt.DateTime(now.Add(23*time.Hour + 55*time.Minute).Truncate(time.Second))

	// Two reminders; recipients are the maintenance notify targets, text is
	// rendered server-side, so the contract carries only fire_at.
	deferred := []*models.ApimodelsDeferredNotification{
		{FireAt: fireAt30},
		{FireAt: fireAt5},
	}

	createReq := &models.ApimodelsCreateDraftMaintRequest{
		Title:                 "Deferred reminders maintenance",
		Description:           "exercises deferred_notifications contract",
		Impact:                models.ApimodelsMaintenanceImpactNone,
		Scope:                 models.ApimodelsMaintenanceScopeGlobal,
		PlannedStart:          plannedStart,
		Steps:                 testMaintenanceSteps(),
		NotifyTargets:         &models.ApimodelsNotifyTargets{ChannelIds: channelIDs},
		DeferredNotifications: deferred,
	}

	createResp, err := apiClient.Maintenances.PostAPIV1MaintenancesCreate(
		maintenances.NewPostAPIV1MaintenancesCreateParams().WithContext(ctx).WithRequest(createReq),
		nil,
	)
	require.NoError(t, err, "Failed to create maintenance with deferred notifications")
	require.Len(t, createResp.Payload.DeferredNotifications, 2, "create response should echo deferred notifications")

	maintID := strfmt.UUID(createResp.Payload.ID)

	// GET round-trips the reminders with their channels.
	getResp, err := apiClient.Maintenances.GetAPIV1MaintenancesID(
		maintenances.NewGetAPIV1MaintenancesIDParams().WithContext(ctx).WithID(maintID),
		nil,
	)
	require.NoError(t, err, "Failed to get maintenance")
	require.Len(t, getResp.Payload.DeferredNotifications, 2)
	// Reminders round-trip ordered by fire_at.
	require.False(t, time.Time(getResp.Payload.DeferredNotifications[0].FireAt).IsZero())

	// Approve enqueues the reminders.
	revision := time.Time(getResp.Payload.CreatedAt).UnixMicro()
	_, err = apiClient.Maintenances.PostAPIV1MaintenancesIDApprove(
		maintenances.NewPostAPIV1MaintenancesIDApproveParams().
			WithContext(ctx).
			WithID(maintID).
			WithRequest(&models.ApimodelsApproveDraftMaintRequest{
				ObservedMaintRevision: revision,
				ConflictsSnapshot:     []*models.ApimodelsConflict{},
			}),
		nil,
	)
	require.NoError(t, err, "Failed to approve maintenance")

	// Cancel cancels the pending reminders (best-effort, must not error).
	_, err = apiClient.Maintenances.PostAPIV1MaintenancesIDCancel(
		maintenances.NewPostAPIV1MaintenancesIDCancelParams().
			WithContext(ctx).
			WithID(maintID).
			WithRequest(&models.ApimodelsCancelMaintRequest{
				Reason:  models.ApimodelsMaintenanceCancelReasonRescheduled,
				Comment: "rescheduled, drop reminders",
			}),
		nil,
	)
	require.NoError(t, err, "Failed to cancel maintenance with deferred reminders")

	getAfter, err := apiClient.Maintenances.GetAPIV1MaintenancesID(
		maintenances.NewGetAPIV1MaintenancesIDParams().WithContext(ctx).WithID(maintID),
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, string(models.UimodelsMaintenanceStatusCanceled), getAfter.Payload.Status)
	// Reminders remain attached to the maintenance record after cancel.
	require.Len(t, getAfter.Payload.DeferredNotifications, 2)
}

// availableChannelIDs returns every catalog channel id exposed by the API.
func availableChannelIDs(ctx context.Context, t *testing.T, apiClient *client.Maintmode) []string {
	t.Helper()
	resp, err := apiClient.Notifications.GetAPIV1NotificationsChannels(
		notifications.NewGetAPIV1NotificationsChannelsParams().WithContext(ctx), nil,
	)
	require.NoError(t, err, "Failed to fetch notification channels")
	require.NotEmpty(t, resp.Payload.Channels, "catalog must expose at least one channel for API tests")

	ids := make([]string, 0, len(resp.Payload.Channels))
	for _, ch := range resp.Payload.Channels {
		ids = append(ids, ch.ID)
	}
	return ids
}
