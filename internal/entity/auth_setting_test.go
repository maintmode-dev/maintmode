package entity_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// The closed set is what keeps a name nothing implements out of the table, and
// it is asserted here as LITERALS rather than against the constants.
//
// Comparing entity.AuthMethodNameEmailOTP against itself would pass through any
// rename and prove nothing. These two strings are also the ids
// GET /auth/providers puts on the wire, so a rename that slipped through would
// change a public contract silently -- writing them out twice, independently,
// is the point.
func TestAuthMethodName_ClosedSet(t *testing.T) {
	t.Parallel()

	require.True(t, entity.AuthMethodName("email_otp").IsValid())
	require.True(t, entity.AuthMethodName("email_password").IsValid())

	for _, invalid := range []string{"", "sms_otp", "bootstrap", "google", "EMAIL_OTP"} {
		require.Falsef(t, entity.AuthMethodName(invalid).IsValid(), "%q must not be valid", invalid)
	}
}

// bootstrap must never be toggleable: it is the break-glass credential, and a
// row for it would be a switch that must never be thrown. Asserted separately
// from the table above because this one is a security property rather than a
// spelling.
func TestAuthMethodName_BootstrapIsNotToggleable(t *testing.T) {
	t.Parallel()

	require.False(t, entity.AuthMethodName("bootstrap").IsValid())

	for _, name := range entity.AllAuthMethodNames() {
		require.NotEqual(t, entity.AuthMethodName("bootstrap"), name)
	}
}

// AllAuthMethodNames and IsValid must agree, or a method could be listed and
// then refused, or validated and never listed.
func TestAllAuthMethodNames_MatchesIsValid(t *testing.T) {
	t.Parallel()

	names := entity.AllAuthMethodNames()
	require.Len(t, names, 2)

	for _, name := range names {
		require.Truef(t, name.IsValid(), "%q is listed but not valid", name)
	}
}

// TestAuditActionAuthMethodToggled_ValidCategorizedAndEntityTyped guards the
// four lookups a new audit action silently depends on.
//
// Each of them fails quietly if missed, and each fails differently:
//   - IsValid: the row is written, renders, and is rejected by the read filter
//     that names it (app/api/public/audit/audit_log.go).
//   - the action -> category map: fillPayload looks the category up BEFORE
//     dispatching, so the row never renders at all.
//   - the category -> actions map: the row exists, renders, and is invisible
//     under the auth filter chip.
//   - EntityType: Renderer.Render defaults it to "user" and every arm of
//     fillAuthPayload relies on that default, so a toggle would be filed
//     against the admin who threw the switch instead of the method.
//
// This is not hypothetical. password.changed and password.reset are in both
// category maps and missing from IsValid() today -- a live instance of the
// first failure mode, and the reason this test exists rather than care.
func TestAuditActionAuthMethodToggled_ValidAndCategorized(t *testing.T) {
	t.Parallel()

	action := entity.AuditActionAuthMethodToggled

	require.True(t, action.IsValid(), "the read filter rejects an action IsValid does not know")

	category, ok := entity.AuditActionCategory(action)
	require.True(t, ok, "without a category the row never renders")
	require.Equal(t, entity.AuditCategoryAuth, category)

	require.Contains(t, entity.AuditCategoryAction(entity.AuditCategoryAuth), action,
		"without this the row is invisible under the auth chip")
}

// The entity type must be its own value: reusing "user" would file the row
// against the admin who threw the switch, making "what happened to this
// instance's sign-in configuration" unanswerable by entity.
func TestAuditEntityTypeAuthSetting_IsDistinct(t *testing.T) {
	t.Parallel()

	require.Equal(t, entity.AuditEntityType("auth_setting"), entity.AuditEntityTypeAuthSetting)
	require.NotEqual(t, entity.AuditEntityTypeUser, entity.AuditEntityTypeAuthSetting)
	require.NotEqual(t, entity.AuditEntityTypeIntegration, entity.AuditEntityTypeAuthSetting)
}
