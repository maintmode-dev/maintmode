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
