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

	// AuditActionPasswordChanged records a user setting or replacing their own
	// password. A security event in its own right: it evicts every other session.
	AuditActionPasswordChanged AuditAction = "password.changed"

	// AuditActionPasswordReset records a password set through a one-time code
	// rather than by proving the old one. It evicts every session.
	AuditActionPasswordReset AuditAction = "password.reset"

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
	// AuditFailureUserBlocked marks a blocked account that got as far as token
	// issuance: the identity verified and the user row resolved, and only then
	// did IssueTokenPair refuse.
	//
	// It is deliberately NOT filed under AuditFailureTokenIssuance, which it
	// would otherwise share a branch with. That reason means "this deployment
	// could not mint a token" — an incident an operator is expected to act on.
	// This one means "the system did exactly what it was configured to do", and
	// collapsing the two would make a run of ordinary blocked-user sign-ins
	// indistinguishable from a failing token service. Same distinction
	// AuditFailureSignupDisabled draws one step earlier, and the same one
	// apperr's ErrInvalidCredentials doc insists on for refused accounts.
	AuditFailureUserBlocked AuditFailureReason = "user blocked"
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
	// AuditFailureSessionMismatch marks a correct-shaped attempt that could not
	// prove it came from the browser the flow began in. It is a risk signal
	// rather than a routine error: the ordinary cause is a user who closed the
	// tab mid-flow, but the same event is what a secret relayed to a third party
	// looks like.
	//
	// It serves two flows, and they establish that proof differently.
	//
	// For one-time codes a nonce travels in the request body and is compared
	// against the one bound to the attempt — a per-attempt value, spent once.
	// The web client calls this backend server-side, so a cookie would bind that
	// server rather than the user's browser.
	//
	// For the OAuth dance there is no stored value to compare against: /start
	// hands the browser an HMAC signature over the state it sent the provider,
	// and this reason records that the signature did not verify — absent,
	// forged, expired, or for a different state or provider. Note what that does
	// NOT mean here: a signature is deterministic, so unlike the nonce it is not
	// a one-shot value, and a mismatch says the pair failed to authenticate
	// rather than that something was spent twice.
	//
	// Different mechanisms, one meaning — the request could not prove its
	// origin — which is why this is one reason and not two.
	AuditFailureSessionMismatch AuditFailureReason = "session nonce mismatch"
	// AuditFailureUnknown covers a rejection whose cause the failing layer could
	// not name -- in practice an infrastructural error, where the request failed
	// before any decision about the credential was made.
	//
	// It exists so that "the trail is silent" never means "nothing happened".
	// An attempt that was refused is worth recording even when the reason is
	// only "something below broke", because the alternative is a gap in the one
	// record an operator reads after an incident.
	AuditFailureUnknown AuditFailureReason = "unknown failure reason"

	// AuditFailurePasswordPolicy is a new password refused for its length.
	//nolint:gosec // G101 false positive: this is an audit reason, not a credential.
	AuditFailurePasswordPolicy AuditFailureReason = "password policy violation"

	// AuditFailureCodeExpired marks a code presented after its expiry.
	//nolint:gosec // G101 false positive: a human-readable failure reason, not a credential
	AuditFailureCodeExpired AuditFailureReason = "code expired"

	// A dance that dies at the state check is pre-identification in the
	// strongest sense in this file: it carries no identity at all, not even a
	// claimed address. Those rows are identified by their metadata — IP and user
	// agent — and their actor fields are zero.
	//
	// There is deliberately no "code reused" value for the dance's one-time
	// code: AuditFailureInvalidCode above already covers an unknown code and
	// losing the race to consume one, which is exactly that case. A second
	// symbol would split one documented meaning across two names.
	//
	// There were once two more reasons here, "oauth state expired" and "oauth
	// state reused", which told an abandoned tab from a replay by consulting a
	// tombstone the store wrote when it consumed a state. The dance no longer
	// stores anything: the state rides in a signed cookie, so a lapsed dance and
	// a replayed URL both arrive as a signature that does not verify, and no
	// mechanism can separate them. Both now record as
	// AuditFailureSessionMismatch. Do not reintroduce the distinction without
	// reintroducing something that can actually observe it.

	// AuditFailureProviderUnavailable marks the provider refusing or failing the
	// back-channel exchange: the token endpoint rejected the code or the
	// client_secret, timed out, or returned an id_token that would not verify.
	//
	// It is its own reason rather than a reuse of AuditFailureInvalidCredentials,
	// which is documented as a password that did not match. The two would be
	// indistinguishable in the trail while meaning opposite things: a run of
	// "invalid credentials" reads as someone guessing passwords, while a run of
	// this one reads as a rotated client_secret or an unreachable Google — a
	// different incident with a different runbook. §10 makes the audit trail the
	// only signal until RUK-292 adds counters, which is exactly why it must not
	// be blurred.
	//
	// It differs from AuditFailureProviderDenied in who ended the dance: that one
	// is the provider telling us up front, in the redirect, that it will not
	// proceed; this one is the back-channel call failing after the browser has
	// already come back to us.
	AuditFailureProviderUnavailable AuditFailureReason = "oauth provider unavailable"
	// AuditFailureProviderDenied marks the provider ending the dance: the user
	// declined consent, or the provider returned an OAuth error of its own.
	//
	// It does not belong to the group above and is not filed under its framing:
	// in the common case nothing failed and nobody is being attacked — a person
	// clicked "cancel". It is recorded because a sudden run of denials usually
	// means a broken consent screen or a misconfigured client, which is
	// invisible otherwise.
	AuditFailureProviderDenied AuditFailureReason = "oauth provider denied"
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

	// Password events are auth, not "block": they are sign-in credential
	// changes, and they belong on the same FE chip as the logins they affect.
	AuditActionPasswordChanged: AuditCategoryAuth,
	AuditActionPasswordReset:   AuditCategoryAuth,

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
		AuditActionPasswordChanged,
		AuditActionPasswordReset,
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
