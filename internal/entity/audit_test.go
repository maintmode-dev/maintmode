package entity

import (
	"testing"

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

// TestAuditActionUserTagsChanged_ValidAndInRolesCategory guards the two lookups
// the admin tag-edit action depends on: IsValid (else the read filter rejects
// it) and the roles category in both directions — the action -> category map
// feeds the facet counts, the reverse map expands the "roles" chip into the
// action filter, so a one-sided entry would count entries the chip then hides.
func TestAuditActionUserTagsChanged_ValidAndInRolesCategory(t *testing.T) {
	require.True(t, AuditActionUserTagsChanged.IsValid())

	category, ok := AuditActionCategory(AuditActionUserTagsChanged)
	require.True(t, ok)
	require.Equal(t, AuditCategoryRoles, category)

	require.Contains(t, AuditCategoryAction(AuditCategoryRoles), AuditActionUserTagsChanged)
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
