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
	"github.com/ruko1202/maintmode/internal/utils/xuuid"

	maintmodeclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/maintmode"
)

func TestUIAPI_GetMaintenanceView(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

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

	// created_by is resolved from auth on read and always present. The test
	// token's subject is not a real auth user, so resolution degrades to the
	// "Unknown user" label — the id is still carried and the read does not fail.
	require.NotNil(t, maint.CreatedBy, "created_by should be present on read")
	require.NotEmpty(t, lo.FromPtr(maint.CreatedBy.Id), "created_by.id should carry the author id")
	require.NotEmpty(t, lo.FromPtr(maint.CreatedBy.DisplayName), "created_by.display_name should be set (resolved or Unknown user)")

	actions := payload.Actions
	require.True(t, lo.FromPtr(actions.CanEdit), "Should be able to edit draft")
	require.True(t, lo.FromPtr(actions.CanApprove), "Should be able to approve draft")
	require.False(t, lo.FromPtr(actions.CanStart), "Should not be able to start draft")
	require.False(t, lo.FromPtr(actions.CanComplete), "Should not be able to finish draft")
}

func TestUIAPI_GetMaintenanceView_Planned(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

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
	ctx := ctxWithLogger(context.Background(), t)

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
	ctx := ctxWithLogger(context.Background(), t)

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
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupMaintmodeTestClient()

	nonExistentID := xuuid.New()

	resp, err := apiClient.GetUiV1MaintenancesIdWithResponse(ctx, nonExistentID)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode(), "Should return not found for non-existent maintenance")
	require.NotNil(t, resp.JSON404, "Error payload should not be nil")
}

// TestUIAPI_GetMaintenanceView_DeferredNotifications pins the FE contract on a
// draft: both reminders come back ordered by fire_at ASC, carry an id and are
// not yet scheduled (the goque tasks are enqueued only on approve).
func TestUIAPI_GetMaintenanceView_DeferredNotifications(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupMaintmodeTestClient()

	maintenanceID, fireAtEarly, fireAtLate := createMaintenanceWithTwoReminders(ctx, t, apiClient)

	view := getMaintenanceView(ctx, t, apiClient, maintenanceID)

	reminders := lo.FromPtr(view.DeferredNotifications)
	require.Len(t, reminders, 2, "draft should expose both reminders")

	// fire_at ASC: the earlier reminder comes first regardless of the order the
	// create request listed them in.
	require.Equal(t, fireAtEarly, lo.FromPtr(reminders[0].FireAt), "reminders must be ordered by fire_at ASC")
	require.Equal(t, fireAtLate, lo.FromPtr(reminders[1].FireAt), "reminders must be ordered by fire_at ASC")

	for i, reminder := range reminders {
		require.NotEqual(t, uuid.Nil, lo.FromPtr(reminder.Id), "reminder %d should carry an id", i)
		require.False(t, lo.FromPtr(reminder.Scheduled), "reminder %d must not be scheduled on a draft", i)
	}
}

// TestUIAPI_GetMaintenanceView_DeferredNotificationsScheduled asserts that
// approving flips `scheduled` to true while leaving the reminder set intact.
func TestUIAPI_GetMaintenanceView_DeferredNotificationsScheduled(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupMaintmodeTestClient()

	maintenanceID, fireAtEarly, fireAtLate := createMaintenanceWithTwoReminders(ctx, t, apiClient)

	approveMaintenance(ctx, t, apiClient, maintenanceID)

	view := getMaintenanceView(ctx, t, apiClient, maintenanceID)
	require.Equal(t, maintmodeclient.MaintenanceStatusPlanned, lo.FromPtr(view.Status))

	reminders := lo.FromPtr(view.DeferredNotifications)
	require.Len(t, reminders, 2, "approve must keep both reminders")
	require.Equal(t, fireAtEarly, lo.FromPtr(reminders[0].FireAt))
	require.Equal(t, fireAtLate, lo.FromPtr(reminders[1].FireAt))

	for i, reminder := range reminders {
		require.True(t, lo.FromPtr(reminder.Scheduled), "reminder %d must be scheduled after approve", i)
	}
}

// TestUIAPI_GetMaintenanceView_DeferredNotificationsEmpty pins the hard contract
// requirement: a maintenance without reminders serializes the field as an empty
// array, never null, so the FE can iterate it unconditionally.
func TestUIAPI_GetMaintenanceView_DeferredNotificationsEmpty(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupMaintmodeTestClient()

	maintenanceID := createTestMaintenance(ctx, t, apiClient)

	view := getMaintenanceView(ctx, t, apiClient, maintenanceID)

	// NotNil on the pointer proves the JSON carried `[]` and not `null`/absent:
	// the generated field is *[]T, so a null or missing value decodes to nil.
	require.NotNil(t, view.DeferredNotifications, "deferred_notifications must be [] and never null")
	require.Empty(t, lo.FromPtr(view.DeferredNotifications))
}

// TestUIAPI_GetMaintenanceView_DeferredNotificationsClearedOnEdit covers the
// clearing semantics: an edit carrying an explicit empty set drops the reminders.
func TestUIAPI_GetMaintenanceView_DeferredNotificationsClearedOnEdit(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupMaintmodeTestClient()

	maintenanceID, _, _ := createMaintenanceWithTwoReminders(ctx, t, apiClient)
	require.Len(t, lo.FromPtr(getMaintenanceView(ctx, t, apiClient, maintenanceID).DeferredNotifications), 2)

	editReq := maintenanceEditRequest(ctx, t, apiClient)
	// `omitempty` keys off the pointer being nil, not the slice length, so a
	// pointer to an empty slice really is marshaled as
	// {"deferred_notifications":[]} — which is the wire value that means "clear".
	// Leaving the field nil would instead mean "do not touch".
	editReq.DeferredNotifications = lo.ToPtr([]maintmodeclient.ApimodelsDeferredNotification{})

	editResp, err := apiClient.PostApiV1MaintenancesIdEditWithResponse(ctx, uuid.MustParse(maintenanceID), editReq)
	require.NoError(t, err, "Failed to edit maintenance with an empty reminder set")
	require.Equal(t, http.StatusNoContent, editResp.StatusCode(), "unexpected status: %s", editResp.Body)

	view := getMaintenanceView(ctx, t, apiClient, maintenanceID)
	require.NotNil(t, view.DeferredNotifications, "deferred_notifications must be [] and never null")
	require.Empty(t, lo.FromPtr(view.DeferredNotifications), "an explicit empty set must clear the reminders")
}

// TestUIAPI_GetMaintenanceView_DeferredNotificationsUntouchedOnEdit is the
// backwards-compatibility guard: an edit that omits the field entirely (clients
// written before reminders existed) must leave the existing reminders alone.
func TestUIAPI_GetMaintenanceView_DeferredNotificationsUntouchedOnEdit(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupMaintmodeTestClient()

	maintenanceID, fireAtEarly, fireAtLate := createMaintenanceWithTwoReminders(ctx, t, apiClient)

	editReq := maintenanceEditRequest(ctx, t, apiClient)
	// Field left nil: `omitempty` drops it from the payload entirely, which the
	// server reads as "unchanged".
	editReq.DeferredNotifications = nil

	editResp, err := apiClient.PostApiV1MaintenancesIdEditWithResponse(ctx, uuid.MustParse(maintenanceID), editReq)
	require.NoError(t, err, "Failed to edit maintenance without the reminders field")
	require.Equal(t, http.StatusNoContent, editResp.StatusCode(), "unexpected status: %s", editResp.Body)

	view := getMaintenanceView(ctx, t, apiClient, maintenanceID)

	reminders := lo.FromPtr(view.DeferredNotifications)
	require.Len(t, reminders, 2, "an edit omitting the field must not touch the reminders")
	require.Equal(t, fireAtEarly, lo.FromPtr(reminders[0].FireAt))
	require.Equal(t, fireAtLate, lo.FromPtr(reminders[1].FireAt))
}

// createMaintenanceWithTwoReminders creates a draft carrying two reminders and
// returns its id together with the two fire_at values, earliest first. The
// reminders are submitted in reverse order so that any test asserting fire_at
// ASC proves the ordering rather than echoing the input order.
//
// The maintenance is scoped to a freshly created (hence unique) resource so it
// stays conflict-free on the shared database and can be approved with an empty
// conflict snapshot.
func createMaintenanceWithTwoReminders(
	ctx context.Context,
	t *testing.T,
	apiClient *maintmodeclient.ClientWithResponses,
) (maintenanceID string, fireAtEarly, fireAtLate time.Time) {
	t.Helper()

	resource := creatResource(ctx, t, apiClient)

	// Truncated to a second because that is the precision the reminders
	// round-trip through the API with.
	plannedStart := xtime.UTCNow().Add(testMaintenanceStartOffset).Truncate(time.Second)
	fireAtEarly = plannedStart.Add(-30 * time.Minute)
	fireAtLate = plannedStart.Add(-5 * time.Minute)

	req := maintmodeclient.PostApiV1MaintenancesCreateJSONRequestBody{
		Title:        lo.ToPtr("Maintenance view reminders " + xuuid.NewString()),
		Description:  lo.ToPtr("Exercises deferred_notifications on the UI view"),
		Impact:       lo.ToPtr(maintmodeclient.MaintenanceImpactNone),
		Scope:        lo.ToPtr(maintmodeclient.MaintenanceScopeResources),
		PlannedStart: lo.ToPtr(plannedStart),
		Resources: lo.ToPtr([]maintmodeclient.ApimodelsResourceRef{
			{Id: lo.ToPtr(uuid.MustParse(lo.FromPtr(resource.Id)))},
		}),
		Steps:         lo.ToPtr(testMaintenanceSteps()),
		NotifyTargets: testNotifyTargets(ctx, t, apiClient),
		DeferredNotifications: lo.ToPtr([]maintmodeclient.ApimodelsDeferredNotification{
			{FireAt: lo.ToPtr(fireAtLate)},
			{FireAt: lo.ToPtr(fireAtEarly)},
		}),
		ApproverUserId: lo.ToPtr(resolveEligibleApprover(ctx, t, apiClient)),
	}

	resp, err := apiClient.PostApiV1MaintenancesCreateWithResponse(ctx, req)
	require.NoError(t, err, "Failed to create maintenance with reminders")
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	return lo.FromPtr(resp.JSON200.Id).String(), fireAtEarly, fireAtLate
}

// maintenanceEditRequest builds a complete edit body. Every field the update
// validator marks Required is filled in; callers only set DeferredNotifications
// to express the case under test.
func maintenanceEditRequest(
	ctx context.Context,
	t *testing.T,
	apiClient *maintmodeclient.ClientWithResponses,
) maintmodeclient.PostApiV1MaintenancesIdEditJSONRequestBody {
	t.Helper()

	resource := creatResource(ctx, t, apiClient)

	return maintmodeclient.PostApiV1MaintenancesIdEditJSONRequestBody{
		Title:        lo.ToPtr("Edited maintenance " + xuuid.NewString()),
		Description:  lo.ToPtr("Edited by the deferred-notifications view tests"),
		Impact:       lo.ToPtr(maintmodeclient.MaintenanceImpactNone),
		Scope:        lo.ToPtr(maintmodeclient.MaintenanceScopeResources),
		PlannedStart: lo.ToPtr(xtime.UTCNow().Add(testMaintenanceStartOffset).Truncate(time.Second)),
		Resources: lo.ToPtr([]maintmodeclient.ApimodelsResourceRef{
			{Id: lo.ToPtr(uuid.MustParse(lo.FromPtr(resource.Id)))},
		}),
		Steps:         lo.ToPtr(testMaintenanceSteps()),
		NotifyTargets: testNotifyTargets(ctx, t, apiClient),
	}
}

// getMaintenanceView fetches the UI read-view of a maintenance and returns its
// maintenance section.
func getMaintenanceView(
	ctx context.Context,
	t *testing.T,
	apiClient *maintmodeclient.ClientWithResponses,
	maintenanceID string,
) *maintmodeclient.UimodelsMaintenanceView {
	t.Helper()

	resp, err := apiClient.GetUiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err, "Failed to get maintenance view")
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)
	require.NotNil(t, resp.JSON200.Maintenance)

	return resp.JSON200.Maintenance
}
