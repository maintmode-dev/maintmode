//go:build api

package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
	maintmodeclient "github.com/ruko1202/maintmode/test/api/client/maintmode"
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

	// Resource-scoped against a freshly created (hence unique) resource: this
	// keeps the maintenance conflict-free regardless of what other parallel
	// tests create, so the empty conflict snapshot on approve stays valid. The
	// scope is irrelevant to the deferred-notifications behavior under test.
	resource := creatResource(ctx, t, apiClient)
	resourceID := uuid.MustParse(lo.FromPtr(resource.Id))

	now := xtime.UTCNow()
	plannedStart := now.Add(24 * time.Hour).Truncate(time.Second)
	fireAt30 := now.Add(23*time.Hour + 30*time.Minute).Truncate(time.Second)
	fireAt5 := now.Add(23*time.Hour + 55*time.Minute).Truncate(time.Second)

	// Two reminders; recipients are the maintenance notify targets, text is
	// rendered server-side, so the contract carries only fire_at.
	deferred := []maintmodeclient.ApimodelsDeferredNotification{
		{FireAt: lo.ToPtr(fireAt30)},
		{FireAt: lo.ToPtr(fireAt5)},
	}

	createReq := maintmodeclient.PostApiV1MaintenancesCreateJSONRequestBody{
		Title:                 lo.ToPtr("Deferred reminders maintenance"),
		Description:           lo.ToPtr("exercises deferred_notifications contract"),
		Impact:                lo.ToPtr(maintmodeclient.MaintenanceImpactNone),
		Scope:                 lo.ToPtr(maintmodeclient.MaintenanceScopeResources),
		PlannedStart:          lo.ToPtr(plannedStart),
		Resources:             lo.ToPtr([]maintmodeclient.ApimodelsResourceRef{{Id: lo.ToPtr(resourceID)}}),
		Steps:                 lo.ToPtr(testMaintenanceSteps()),
		NotifyTargets:         &maintmodeclient.ApimodelsNotifyTargets{ChannelIds: lo.ToPtr(channelIDs)},
		DeferredNotifications: lo.ToPtr(deferred),
	}

	createResp, err := apiClient.PostApiV1MaintenancesCreateWithResponse(ctx, createReq)
	require.NoError(t, err, "Failed to create maintenance with deferred notifications")
	require.Equal(t, http.StatusOK, createResp.StatusCode(), "unexpected status: %s", createResp.Body)
	require.NotNil(t, createResp.JSON200)
	require.Len(t, lo.FromPtr(createResp.JSON200.DeferredNotifications), 2, "create response should echo deferred notifications")

	maintID := lo.FromPtr(createResp.JSON200.Id)

	// GET round-trips the reminders with their channels.
	getResp, err := apiClient.GetApiV1MaintenancesIdWithResponse(ctx, maintID)
	require.NoError(t, err, "Failed to get maintenance")
	require.Equal(t, http.StatusOK, getResp.StatusCode(), "unexpected status: %s", getResp.Body)
	require.NotNil(t, getResp.JSON200)
	require.Len(t, lo.FromPtr(getResp.JSON200.DeferredNotifications), 2)
	// Reminders round-trip ordered by fire_at.
	require.False(t, lo.FromPtr(lo.FromPtr(getResp.JSON200.DeferredNotifications)[0].FireAt).IsZero())

	// Approve enqueues the reminders. The maintenance is conflict-free (unique
	// resource), so the empty conflict snapshot is accepted.
	approveMaintenance(ctx, t, apiClient, maintID.String())

	// Cancel cancels the pending reminders (best-effort, must not error).
	cancelResp, err := apiClient.PostApiV1MaintenancesIdCancelWithResponse(ctx, maintID,
		maintmodeclient.PostApiV1MaintenancesIdCancelJSONRequestBody{
			Reason:  lo.ToPtr(maintmodeclient.MaintenanceCancelReasonRescheduled),
			Comment: lo.ToPtr("rescheduled, drop reminders"),
		})
	require.NoError(t, err, "Failed to cancel maintenance with deferred reminders")
	require.Equal(t, http.StatusNoContent, cancelResp.StatusCode(), "unexpected status: %s", cancelResp.Body)

	getAfter, err := apiClient.GetApiV1MaintenancesIdWithResponse(ctx, maintID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, getAfter.StatusCode(), "unexpected status: %s", getAfter.Body)
	require.NotNil(t, getAfter.JSON200)
	require.Equal(t, string(maintmodeclient.MaintenanceStatusCancelled), lo.FromPtr(getAfter.JSON200.Status))
	// Reminders remain attached to the maintenance record after cancel.
	require.Len(t, lo.FromPtr(getAfter.JSON200.DeferredNotifications), 2)
}

// availableChannelIDs returns every catalog channel id exposed by the API,
// seeding one via the admin API first so the catalog is never empty.
func availableChannelIDs(ctx context.Context, t *testing.T, apiClient *maintmodeclient.ClientWithResponses) []string {
	t.Helper()

	ensureNotifyChannel(ctx, t, apiClient)

	resp, err := apiClient.GetApiV1NotificationsChannelsWithResponse(ctx, nil)
	require.NoError(t, err, "Failed to fetch notification channels")
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	channels := lo.FromPtr(resp.JSON200.Channels)
	require.NotEmpty(t, channels, "catalog must expose at least one channel for API tests")

	ids := make([]string, 0, len(channels))
	for _, ch := range channels {
		ids = append(ids, lo.FromPtr(ch.Id).String())
	}
	return ids
}
