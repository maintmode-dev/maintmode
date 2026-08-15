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

	maintmodeclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/maintmode"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// TestMaintenancesAPI_DeferredNotifications walks the full deferred-reminder
// lifecycle through the public API: create a draft carrying two reminders (one
// fanned out to two channels), confirm they round-trip on GET, approve (which
// enqueues the goque reminders), then cancel (which cancels them). The flow
// exercises the server-side enqueue/cancel path end to end.
func TestMaintenancesAPI_DeferredNotifications(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
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
		ApproverUserId:        lo.ToPtr(resolveEligibleApprover(ctx, t, apiClient)),
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

// availableChannelIDs returns notify-target channel ids the caller can use,
// seeded here rather than read back from the catalog.
//
// It used to seed one channel and then list the catalog. Now that the listing is
// paged that read returns page 0 — fifty channels ordered by
// (transport, transport_channel_id), which on a shared database almost never
// includes the row just created. The test would have gone on passing, on other
// tests' data. Returning what we seeded keeps the assertion about this test's
// own rows.
func availableChannelIDs(ctx context.Context, t *testing.T, apiClient *maintmodeclient.ClientWithResponses) []string {
	t.Helper()

	return []string{ensureNotifyChannel(ctx, t, apiClient)}
}
