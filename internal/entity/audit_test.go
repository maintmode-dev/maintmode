package entity

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

// TestAuditActionWireValues pins the exact on-the-wire string of every audit
// action. These values are a contract: they land in audit_log.action, the read
// filter, and the generated swagger client. A rename here that isn't mirrored in
// the regenerated client (or vice versa) silently breaks FE filtering — this
// test is the guard. Update it deliberately when the contract changes.
func TestAuditActionWireValues(t *testing.T) {
	want := map[AuditAction]string{
		AuditActionLoginSuccess:    "login.success",
		AuditActionLoginFailed:     "login.failed",
		AuditActionLogoutSuccess:   "logout.success",
		AuditActionRolesChanged:    "roles.changed",
		AuditActionUserBlocked:     "user.blocked",
		AuditActionUserUnblocked:   "user.unblocked",
		AuditActionUserTagsChanged: "user.tags_changed",

		AuditActionMaintCreated:   "maintenance.created",
		AuditActionMaintUpdated:   "maintenance.updated",
		AuditActionMaintApproved:  "maintenance.approved",
		AuditActionMaintStarted:   "maintenance.started",
		AuditActionMaintCompleted: "maintenance.completed",
		AuditActionMaintCanceled:  "maintenance.canceled",

		AuditActionMaintStepStarted:   "maintenance_step.started",
		AuditActionMaintStepCompleted: "maintenance_step.completed",
		AuditActionMaintStepCanceled:  "maintenance_step.canceled",

		AuditActionInvitationCreated: "invitation.created",
		AuditActionInvitationRevoked: "invitation.revoked",

		AuditActionResourceCreated:    "resource.created",
		AuditActionResourceUpdated:    "resource.updated",
		AuditActionResourceArchived:   "resource.archived",
		AuditActionResourceUnarchived: "resource.unarchived",

		AuditActionNotifyChannelCreated:    "notify_channel.created",
		AuditActionNotifyChannelUpdated:    "notify_channel.updated",
		AuditActionNotifyChannelArchived:   "notify_channel.archived",
		AuditActionNotifyChannelUnarchived: "notify_channel.unarchived",
	}

	for action, str := range want {
		require.Equal(t, str, string(action))
	}
}

// TestAuditMaintActions_ValidAndCategorized guards that every maintenance action
// passes IsValid (else the read filter rejects it) and maps to a category (else
// it falls into the get_logs "unknown category" branch).
func TestAuditMaintActions_ValidAndCategorized(t *testing.T) {
	maintActions := []AuditAction{
		AuditActionMaintCreated,
		AuditActionMaintUpdated,
		AuditActionMaintApproved,
		AuditActionMaintStarted,
		AuditActionMaintCompleted,
		AuditActionMaintCanceled,
		AuditActionMaintStepStarted,
		AuditActionMaintStepCompleted,
		AuditActionMaintStepCanceled,
	}

	for _, action := range maintActions {
		require.Truef(t, action.IsValid(), "%q must be valid", action)
		category, ok := AuditActionCategory(action)
		require.Truef(t, ok, "%q must have a category", action)
		require.Equalf(t, AuditCategoryMaintenance, category, "%q must be in the maintenance category", action)
	}
}

// TestAuditActionCategories_Pinned pins which chip every action belongs to.
//
// The category is product copy, not an implementation detail: it decides which
// filter chip an operator must open to find a row, and the UI mirrors this
// table action for action (maintmode-ui src/domain/audit/audit-presentation.ts).
// A move between categories must therefore be a deliberate edit here, never a
// side effect of touching the maps. The expected values are literal strings so
// a renamed constant cannot carry the test along with it.
//
// It also enforces the grouping's one structural rule: every declared action is
// in exactly one category, and no category exists beyond the four chips.
func TestAuditActionCategories_Pinned(t *testing.T) {
	t.Parallel()

	want := map[AuditAction]string{
		AuditActionLoginSuccess:  "sign_in",
		AuditActionLoginFailed:   "sign_in",
		AuditActionLogoutSuccess: "sign_in",

		AuditActionRolesChanged:    "users",
		AuditActionUserTagsChanged: "users",
		AuditActionUserBlocked:     "users",
		AuditActionUserUnblocked:   "users",
		AuditActionPasswordChanged: "users",
		AuditActionPasswordReset:   "users",
		AuditActionProviderLinked:  "users",

		AuditActionAuthMethodToggled:  "settings",
		AuditActionIntegrationCreated: "settings",
		AuditActionIntegrationUpdated: "settings",
		AuditActionIntegrationDeleted: "settings",

		AuditActionInvitationCreated: "users",
		AuditActionInvitationRevoked: "users",

		AuditActionResourceCreated:         "settings",
		AuditActionResourceUpdated:         "settings",
		AuditActionResourceArchived:        "settings",
		AuditActionResourceUnarchived:      "settings",
		AuditActionNotifyChannelCreated:    "settings",
		AuditActionNotifyChannelUpdated:    "settings",
		AuditActionNotifyChannelArchived:   "settings",
		AuditActionNotifyChannelUnarchived: "settings",

		AuditActionMaintCreated:       "maintenance",
		AuditActionMaintUpdated:       "maintenance",
		AuditActionMaintApproved:      "maintenance",
		AuditActionMaintStarted:       "maintenance",
		AuditActionMaintCompleted:     "maintenance",
		AuditActionMaintCanceled:      "maintenance",
		AuditActionMaintStepStarted:   "maintenance",
		AuditActionMaintStepCompleted: "maintenance",
		AuditActionMaintStepCanceled:  "maintenance",
	}

	require.Len(t, want, len(allAuditActions()), "the pin table must list every declared action")
	require.Len(t, auditActionCategories, len(want), "the forward map categorizes an action the pin table does not know")

	for _, action := range allAuditActions() {
		got, ok := AuditActionCategory(action)
		require.Truef(t, ok, "%q has no category", action)
		require.Equalf(t, want[action], string(got), "%q is in the wrong category", action)
	}

	require.ElementsMatch(t,
		[]AuditCategory{"sign_in", "users", "settings", "maintenance"},
		lo.Keys(auditCategoriesAction),
		"the reverse map must carry exactly the four chip categories")

	seen := make(map[AuditAction]AuditCategory, len(want))
	for category, actions := range auditCategoriesAction {
		for _, action := range actions {
			prev, dup := seen[action]
			require.Falsef(t, dup, "%q is listed under both %q and %q", action, prev, category)
			seen[action] = category
		}
	}
	require.Len(t, seen, len(want), "the reverse map must list every action exactly once")
}

// The two direction maps are maintained by hand and are consumed by different
// callers: auditActionCategories answers "which chip does this event belong
// to", auditCategoriesAction answers "which events does this chip select".
// Adding an action to one and forgetting the other is silent -- the event is
// written and categorized correctly, and then simply never appears when a user
// filters by its own category.
//
// This is not hypothetical: the RUK-289 password events were added to the
// forward map and missed in the reverse one.
func TestAuditCategoryMapsAgree(t *testing.T) {
	t.Parallel()

	for action, category := range auditActionCategories {
		require.Contains(t, AuditCategoryAction(category), action,
			"action %q is categorized as %q but that category does not list it, "+
				"so filtering by it will not return this event", action, category)
	}

	for category, actions := range auditCategoriesAction {
		for _, action := range actions {
			got, ok := AuditActionCategory(action)
			require.True(t, ok, "category %q lists an action with no category of its own: %q",
				category, action)
			require.Equal(t, category, got,
				"action %q is listed under %q but categorized as %q", action, category, got)
		}
	}
}

// allAuditActions is every action this package declares.
//
// Hand-maintained, like the three lists it exists to check, and that is the
// point: a new action is not covered until someone adds it here, and the
// neighboring TestAuditActionWireValues fails the same way. Two lists that
// must be edited together are worth more than one list that silently skips
// what it does not know about -- a reflected or derived list would grow
// automatically and assert nothing about the entry it just grew.
func allAuditActions() []AuditAction {
	return []AuditAction{
		AuditActionLoginSuccess,
		AuditActionLoginFailed,
		AuditActionLogoutSuccess,
		AuditActionPasswordChanged,
		AuditActionPasswordReset,
		AuditActionProviderLinked,
		AuditActionRolesChanged,
		AuditActionUserBlocked,
		AuditActionUserUnblocked,
		AuditActionUserTagsChanged,
		AuditActionMaintCreated,
		AuditActionMaintUpdated,
		AuditActionMaintApproved,
		AuditActionMaintStarted,
		AuditActionMaintCompleted,
		AuditActionMaintCanceled,
		AuditActionMaintStepStarted,
		AuditActionMaintStepCompleted,
		AuditActionMaintStepCanceled,
		AuditActionIntegrationCreated,
		AuditActionIntegrationUpdated,
		AuditActionIntegrationDeleted,
		AuditActionAuthMethodToggled,
		AuditActionInvitationCreated,
		AuditActionInvitationRevoked,
		AuditActionResourceCreated,
		AuditActionResourceUpdated,
		AuditActionResourceArchived,
		AuditActionResourceUnarchived,
		AuditActionNotifyChannelCreated,
		AuditActionNotifyChannelUpdated,
		AuditActionNotifyChannelArchived,
		AuditActionNotifyChannelUnarchived,
	}
}

// TestEveryAuditAction_IsValidAndCategorized checks all three hand-maintained
// lists at once, for every action.
//
// The per-action tests below came one at a time, each written after a specific
// action was found missing from a specific list. This one exists because that
// happened three times: the category maps carry comments warning that a missing
// entry fails silently, IsValid carries none, and password.changed and
// password.reset were in both maps and absent from IsValid -- written, rendered,
// and rejected by the read filter that names them.
//
// Each list fails differently, which is why all three are asserted together:
//   - IsValid: the row exists and renders, and the filter refuses the action
//     name (app/api/public/audit/audit_log.go).
//   - action -> category: fillPayload looks the category up BEFORE dispatching,
//     so the row never renders at all.
//   - category -> actions: the row renders and is invisible under its chip.
func TestEveryAuditAction_IsValidAndCategorized(t *testing.T) {
	t.Parallel()

	for _, action := range allAuditActions() {
		t.Run(string(action), func(t *testing.T) {
			t.Parallel()

			require.Truef(t, action.IsValid(),
				"%q is not in the IsValid allow-list, so the read filter rejects it", action)

			category, ok := AuditActionCategory(action)
			require.Truef(t, ok, "%q has no category, so it never renders", action)

			require.Containsf(t, AuditCategoryAction(category), action,
				"%q is missing from the %q action list, so it is invisible under that chip",
				action, category)
		})
	}
}
