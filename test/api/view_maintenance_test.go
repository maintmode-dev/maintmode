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

	"github.com/ruko1202/maintmode/internal/entity"
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
	// The default test client is an admin, and an admin may approve any draft
	// regardless of who it is assigned to.
	require.True(t, lo.FromPtr(actions.CanApprove), "Admin should be able to approve any draft")
	require.False(t, lo.FromPtr(actions.CanStart), "Should not be able to start draft")
	require.False(t, lo.FromPtr(actions.CanComplete), "Should not be able to finish draft")
}

// can_approve mirrors the service gate: the maintenance.approve right AND
// (assigned approver OR admin). A reviewer holds the right but is bound to the
// assignment, so they must not be offered a button that would 403.
func TestUIAPI_GetMaintenanceView_CanApproveIsAssignmentAware(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupMaintmodeTestClient()

	maintenanceID := createTestMaintenance(ctx, t, apiClient)

	getResp, err := apiClient.GetApiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, getResp.StatusCode(), "unexpected status: %s", getResp.Body)
	approverID := lo.FromPtr(lo.FromPtr(getResp.JSON200.Approver).Id)

	otherReviewerClient, _ := provisionUser(ctx, t, entity.RoleReviewer)
	otherAdminClient, _ := provisionUser(ctx, t, entity.RoleAdmin)

	for _, tc := range []struct {
		name          string
		client        *maintmodeclient.ClientWithResponses
		expCanApprove bool
	}{
		{
			name: "assigned reviewer",
			client: setupMaintmodeTestClientWithToken(
				mustTestAccessTokenForUser(approverID.String(), entity.RoleReviewer),
			),
			expCanApprove: true,
		},
		{
			name:          "reviewer assigned to someone else",
			client:        otherReviewerClient,
			expCanApprove: false,
		},
		{
			name:          "admin assigned to someone else",
			client:        otherAdminClient,
			expCanApprove: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := tc.client.GetUiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)

			require.Equal(t, tc.expCanApprove, lo.FromPtr(resp.JSON200.Actions.CanApprove))
		})
	}
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

	// Search rather than index: conflicts are ordered unreviewed-first, and the
	// shared database can contribute unrelated global-scope maintenances, so
	// position 0 is not this maintenance's.
	conflict, found := lo.Find(lo.FromPtr(payload.Conflicts), func(c maintmodeclient.UimodelsConflictView) bool {
		return lo.FromPtr(c.MaintenanceId).String() == maintenanceID1
	})
	require.True(t, found, "conflicts should reference maintenance %s", maintenanceID1)
	require.NotNil(t, conflict.Resources, "Conflict resources should not be nil")

	// NotNil before FromPtr, deliberately: the generated client types the flag as
	// *bool because swag emits no required block for response schemas, so a field
	// that stopped being serialized would read as false and pass silently.
	require.NotNil(t, conflict.KnownAtApproval, "the flag must be present on every conflict")

	// The viewed maintenance is still a draft: nothing has been approved, so no
	// conflict can be flagged as reviewed.
	require.False(t, lo.FromPtr(conflict.KnownAtApproval), "a draft has no approval snapshot")
}

// TestUIAPI_GetMaintenanceView_ConflictsOrderedUnreviewedFirst pins the response
// contract: the flag is present on every conflict, and every unreviewed one
// precedes every reviewed one.
//
// The end-to-end "approver saw this exact conflict" path is not asserted here.
// Approving a maintenance that has conflicts means echoing the live conflict set
// back, and checkConflicts rejects the approval if that set moved in between. On
// the shared database it does: every create helper hardcodes planned_start to
// now+24h (testMaintenanceStartOffset), so all parallel tests land in the same
// window, and global-scope maintenances conflict with everything regardless of
// resource.
//
// This is fixable rather than impossible — placing the subject and its neighbor
// in a per-run unique window far in the future would isolate them — but it needs
// the shared create helpers to accept a planned_start, which is a wider change
// than this test warrants. The Save-to-GetSnapshots round trip it would exercise
// is covered against a real database by
// internal/services/conflicts/test/mark_approval_state_test.go.
func TestUIAPI_GetMaintenanceView_ConflictsOrderedUnreviewedFirst(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupMaintmodeTestClient()

	resourceID := lo.FromPtr(creatResource(ctx, t, apiClient).Id)

	// Only planned and in-progress maintenances count as conflicts, so the
	// neighbor has to be approved to be visible at all.
	neighborID := createMaintenanceWithResource(ctx, t, apiClient, resourceID)
	approveMaintenance(ctx, t, apiClient, neighborID)

	subjectID := createMaintenanceWithResource(ctx, t, apiClient, resourceID)

	resp, err := apiClient.GetUiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(subjectID))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	conflicts := lo.FromPtr(resp.JSON200.Conflicts)
	require.NotEmpty(t, conflicts, "the approved neighbor should conflict")

	_, found := lo.Find(conflicts, func(c maintmodeclient.UimodelsConflictView) bool {
		return lo.FromPtr(c.MaintenanceId).String() == neighborID
	})
	require.True(t, found, "conflicts should reference maintenance %s", neighborID)

	seenReviewed := false
	for _, c := range conflicts {
		require.NotNil(t, c.KnownAtApproval, "the flag must be present on every conflict")

		if lo.FromPtr(c.KnownAtApproval) {
			seenReviewed = true
			continue
		}
		require.False(t, seenReviewed, "an unreviewed conflict must not follow a reviewed one")
	}
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
