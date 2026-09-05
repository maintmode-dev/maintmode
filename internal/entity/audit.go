package entity

import (
	"time"

	"github.com/google/uuid"
)

// AuditAction describes the event type of an audit log entry.
type AuditAction string

const (
	AuditActionLoginSuccess  AuditAction = "login.success"
	AuditActionLoginFailed   AuditAction = "login.failed"
	AuditActionLogoutSuccess AuditAction = "logout.success"

	AuditActionRolesChanged AuditAction = "roles.changed"

	AuditActionUserBlocked   AuditAction = "user.blocked"
	AuditActionUserUnblocked AuditAction = "user.unblocked"

	// AuditActionUserTagsChanged records an admin editing another user's
	// messenger tags. Self-service edits of one's own tags are deliberately not
	// audited — see the audit.UserTagsChanged doc block.
	AuditActionUserTagsChanged AuditAction = "user.tags_changed"

	// Maintenance lifecycle/CRUD actions. AuditActionMaintAutoCanceled
	// is the automatic overdue-cancel path — system actor, no human.
	// Values use the project's "canceled" spelling (matches MaintenanceStatusCancelled = "canceled").
	AuditActionMaintCreated   AuditAction = "maintenance.created"
	AuditActionMaintUpdated   AuditAction = "maintenance.updated"
	AuditActionMaintApproved  AuditAction = "maintenance.approved"
	AuditActionMaintStarted   AuditAction = "maintenance.started"
	AuditActionMaintCompleted AuditAction = "maintenance.completed"
	AuditActionMaintCanceled  AuditAction = "maintenance.canceled"

	AuditActionMaintStepStarted   AuditAction = "maintenance_step.started"
	AuditActionMaintStepCompleted AuditAction = "maintenance_step.completed"
	AuditActionMaintStepCanceled  AuditAction = "maintenance_step.canceled"

	// Integration registry actions. The payload records the kind and
	// enabled flag only — never secret values.
	AuditActionIntegrationCreated AuditAction = "integration.created"
	AuditActionIntegrationUpdated AuditAction = "integration.updated"
)

func (a AuditAction) IsValid() bool {
	switch a {
	case AuditActionLoginSuccess,
		AuditActionLoginFailed,
		AuditActionLogoutSuccess,
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
		AuditActionIntegrationUpdated:
		return true
	default:
		return false
	}
}

type AuditEntityType string

const (
	AuditEntityTypeUser        AuditEntityType = "user"
	AuditEntityTypeMaintenance AuditEntityType = "maintenance"
	AuditEntityTypeIntegration AuditEntityType = "integration"
)

// AuditEntry represents a structured audit log record.
//
// Design:
//   - EntityType + EntityID bind the record to a concrete entity for fast lookup.
//     NOT a foreign key — the entity may be deleted, the audit trail stays.
//   - EntityID is stored as a string (not a UUID) so it does not break when the
//     entity is deleted and can carry different ID kinds (UUID, string name, int).
type AuditEntry struct {
	ID               uuid.UUID
	EventID          uuid.UUID // per-event idempotency key; uuid.Nil for legacy/non-outbox writes
	Action           AuditAction
	Actor            string          // who performed the action (email)
	ActorID          string          // stable actor ID (user UUID, string — not an FK); empty for system
	ActorDisplayName string          // snapshot of the actor name at event time (not resolved on read)
	EntityID         string          // ID of the primary entity (string, not an FK)
	EntityType       AuditEntityType // type of the primary entity: user, maint, etc
	Details          string          // human-readable description
	Metadata         *AuditMetadata  // structured action-specific payload (optional)
	CreatedAt        time.Time
}

type AuditFailureReason string

// Whitelist-safe login failure reasons for audit metadata. The raw error text is
// never written to the audit trail — it may carry internal details.
const (
	AuditFailureUserProvisioning AuditFailureReason = "user provisioning failed"
	//nolint:gosec // G101 false positive: a human-readable failure reason, not a credential
	AuditFailureTokenIssuance AuditFailureReason = "token issuance failed"
	// AuditFailureSignupDisabled marks an OAuth login of an unknown user rejected
	// because neither an invitation nor open signup authorized creating the account.
	AuditFailureSignupDisabled AuditFailureReason = "signup disabled"
	// AuditFailureInvalidCredentials marks a password that did not match. Unlike
	// the three reasons above it is PRE-identification: there is no verified
	// identity behind the attempt, only a claim. It is deliberately its own
	// value — the response to a bad password is indistinguishable from every
	// other failure, so the audit trail is where a wrong password has to remain
	// tellable from a refused or blocked account.
	//nolint:gosec // G101 false positive: a human-readable failure reason, not a credential
	AuditFailureInvalidCredentials AuditFailureReason = "invalid credentials"

	// The one-time-code reasons below are all PRE-identification in the same
	// sense as AuditFailureInvalidCredentials: the verify endpoint answers every
	// failure identically, so the audit trail is the only place they stay
	// tellable apart. They are four values rather than one because an operator
	// reading a burst of them needs to know which: a run of wrong codes is a
	// brute-force attempt, a run of expiries is usually mail being slow, and a
	// run of nonce mismatches is neither.

	// AuditFailureInvalidCode marks a submitted code that did not match, and
	// also the cases indistinguishable from it to the caller: no live code, and
	// losing the race to consume one.
	//nolint:gosec // G101 false positive: a human-readable failure reason, not a credential
	AuditFailureInvalidCode AuditFailureReason = "invalid code"
	// AuditFailureAttemptsExhausted marks a guess refused because the code had
	// already spent its ceiling. The code itself is never compared.
	AuditFailureAttemptsExhausted AuditFailureReason = "attempts exhausted"
	// AuditFailureSessionMismatch marks a correct-shaped attempt whose session
	// nonce did not match the one bound to the code. It is a risk signal rather
	// than a routine error: the ordinary cause is a user who closed the tab
	// while the mail was in flight, but the same event is what a code relayed to
	// a third party looks like.
	AuditFailureSessionMismatch AuditFailureReason = "session nonce mismatch"
	// AuditFailureCodeExpired marks a code presented after its expiry.
	//nolint:gosec // G101 false positive: a human-readable failure reason, not a credential
	AuditFailureCodeExpired AuditFailureReason = "code expired"
)

type AuditLogoutKind string

const (
	AuditLogoutKindManual = "manual"
	AuditLogoutKindAuto   = "auto"
)

// AuditMetadata is the structured, action-specific payload of an audit record.
// Strictly a whitelist of safe fields: IP, user agent, session id, role names.
// NEVER put tokens, cookies, secrets or raw payloads here: the audit trail is
// durable, is never redacted, and every admin can read it back through the API.
//
// Which fields are populated depends on the action:
//   - login_success / login_failed: IP, UserAgent, SessionID (+FailureReason for failed);
//   - logout_success: SessionID, LogoutKind;
//   - assigned / revoked: Roles, TargetEmail, TargetDisplayName;
//   - replaced: Roles (resulting set), RolesAdded, RolesRemoved, TargetEmail, TargetDisplayName;
//   - blocked / unblocked: TargetEmail, TargetDisplayName;
//   - user.tags_changed: Changes (before/after per changed tag), TargetEmail,
//     TargetDisplayName.
type AuditMetadata struct {
	IP                string             `json:"ip,omitempty"`
	UserAgent         string             `json:"user_agent,omitempty"`
	SessionID         string             `json:"session_id,omitempty"`
	FailureReason     AuditFailureReason `json:"failure_reason,omitempty"`
	LogoutKind        AuditLogoutKind    `json:"logout_kind,omitempty"`
	Roles             []string           `json:"roles,omitempty"`
	RolesAdded        []string           `json:"roles_added,omitempty"`
	RolesRemoved      []string           `json:"roles_removed,omitempty"`
	TargetEmail       string             `json:"target_email,omitempty"`
	TargetDisplayName string             `json:"target_display_name,omitempty"`

	// Maintenance action fields. All omitempty; populated only for
	// maintenance.* / maintenance_step.* actions:
	//   - maintenance.* / maintenance_step.*: MaintTitle;
	//   - maintenance.updated: Changes (before/after per changed scalar).
	// The cancel reason is not duplicated here — it lives on the maintenance's own
	// cancel_reason column; the audit row carries it only in the Details string.
	MaintTitle string             `json:"maint_title,omitempty"`
	Changes    []AuditFieldChange `json:"changes,omitempty"`
}

// AuditFieldChange is one before/after entry in a diff (maintenance.updated,
// user.tags_changed). Old/New are rendered string snapshots of a scalar field
// (title, planned window, scope, impact, approver; messenger tags). An empty
// Old or New means the field was unset on that side. Collection fields (steps,
// targets) record a changed flag via Field with empty Old/New rather than noisy
// element diffs.
type AuditFieldChange struct {
	Field string `json:"field"`
	Old   string `json:"old,omitempty"`
	New   string `json:"new,omitempty"`
}

// AuditCategory groups audit actions into the FE filter chips
// (Auth / Roles / Block / Maintenance). The category -> actions mapping is owned
// by the backend so facet counts and category expansion stay consistent.
type AuditCategory string

const (
	AuditCategoryAuth        AuditCategory = "auth"
	AuditCategoryRoles       AuditCategory = "roles"
	AuditCategoryBlock       AuditCategory = "block"
	AuditCategoryMaintenance AuditCategory = "maintenance"
	AuditCategoryIntegration AuditCategory = "integration"
)

var auditActionCategories = map[AuditAction]AuditCategory{
	AuditActionLoginSuccess:  AuditCategoryAuth,
	AuditActionLoginFailed:   AuditCategoryAuth,
	AuditActionLogoutSuccess: AuditCategoryAuth,

	// user.tags_changed rides the roles category on purpose. Categories are the
	// FE filter chips (see AuditCategory) and are fanned out by a switch in
	// services/auditor/get_logs.go; a new category would need both that switch
	// updated and a new chip in a UI that never asked for one. Roles is the
	// closest fit in meaning — "an admin manages someone else's profile". Not
	// Block: this is not a blocking action.
	AuditActionRolesChanged:    AuditCategoryRoles,
	AuditActionUserTagsChanged: AuditCategoryRoles,

	AuditActionUserBlocked:   AuditCategoryBlock,
	AuditActionUserUnblocked: AuditCategoryBlock,

	AuditActionMaintCreated:       AuditCategoryMaintenance,
	AuditActionMaintUpdated:       AuditCategoryMaintenance,
	AuditActionMaintApproved:      AuditCategoryMaintenance,
	AuditActionMaintStarted:       AuditCategoryMaintenance,
	AuditActionMaintCompleted:     AuditCategoryMaintenance,
	AuditActionMaintCanceled:      AuditCategoryMaintenance,
	AuditActionMaintStepStarted:   AuditCategoryMaintenance,
	AuditActionMaintStepCompleted: AuditCategoryMaintenance,
	AuditActionMaintStepCanceled:  AuditCategoryMaintenance,

	AuditActionIntegrationCreated: AuditCategoryIntegration,
	AuditActionIntegrationUpdated: AuditCategoryIntegration,
}

// AuditActionCategory returns the facet category of action.
// ok is false for actions outside the known set.
func AuditActionCategory(action AuditAction) (AuditCategory, bool) {
	category, ok := auditActionCategories[action]
	return category, ok
}

var auditCategoriesAction = map[AuditCategory][]AuditAction{
	AuditCategoryAuth: {
		AuditActionLoginSuccess,
		AuditActionLoginFailed,
		AuditActionLogoutSuccess,
	},
	AuditCategoryRoles: {
		AuditActionRolesChanged,
		AuditActionUserTagsChanged,
	},
	AuditCategoryBlock: {
		AuditActionUserBlocked,
		AuditActionUserUnblocked,
	},
	AuditCategoryMaintenance: {
		AuditActionMaintCreated,
		AuditActionMaintUpdated,
		AuditActionMaintApproved,
		AuditActionMaintStarted,
		AuditActionMaintCompleted,
		AuditActionMaintCanceled,
		AuditActionMaintStepStarted,
		AuditActionMaintStepCompleted,
		AuditActionMaintStepCanceled,
	},
	AuditCategoryIntegration: {
		AuditActionIntegrationCreated,
		AuditActionIntegrationUpdated,
	},
}

func AuditCategoryAction(category AuditCategory) []AuditAction {
	return auditCategoriesAction[category]
}

// AuditFilter is a read-time filter for audit log entries.
// All fields are optional; a nil pointer / empty slice means
// "do not filter by this field".
type AuditFilter struct {
	Actions     []AuditAction
	Actor       *string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
}

// WithoutActions returns a copy of the filter with the action filter dropped.
// Facet counts are computed in the actor/date window regardless of the
// selected category, so the chips keep wayfinding numbers.
func (f *AuditFilter) WithoutActions() *AuditFilter {
	if f == nil {
		return nil
	}
	clone := *f
	clone.Actions = nil
	return &clone
}

// AuditFacets carries per-category entry counts within the current
// actor/date filter window. All is the count across every action.
type AuditFacets struct {
	All         int64
	Auth        int64
	Roles       int64
	Block       int64
	Maintenance int64
	Integration int64
}

// AuditLogsPage is one page of audit log entries plus pagination/facet
// metadata computed under the same filter.
type AuditLogsPage struct {
	Logs   []*AuditEntry
	Total  int64
	Facets AuditFacets
}
