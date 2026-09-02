package entity

import (
	"time"

	"github.com/google/uuid"
)

// --- Maintenance commands ---

type MaintenanceStepInput struct {
	Order               int32
	Description         string
	RollbackDescription string
	DurationMinutes     int64
}

type NotifyTargetInput struct {
	// ChannelID is the catalog channel id (the messenger_channels row UUID),
	// resolved to a concrete transport target during maintenance create/update.
	ChannelID uuid.UUID
}

// MentionInput is one entry of the create/update contract's mentions array:
// a MaintMode user tagged alongside the owner in the maintenance notification.
// An object rather than a bare uuid so the contract can gain keys without a
// breaking change, as with NotifyTargetInput above.
type MentionInput struct {
	UserID uuid.UUID
}

// DeferredNotificationInput is one entry of the create/update contract's
// deferred_notifications array: a reminder to fire at FireAt. Recipients and
// text are not part of the input — reminders go to the maintenance's notify
// targets and are rendered from the maint.reminder template at send time.
type DeferredNotificationInput struct {
	FireAt time.Time
}

type CreateMaintenanceCmd struct {
	Title                 string
	Description           string
	PlannedPeriod         Period
	Scope                 MaintenanceScope
	Impact                MaintenanceImpact
	Resources             []uuid.UUID
	Steps                 []*MaintenanceStepInput
	NotifyTargets         []*NotifyTargetInput
	DeferredNotifications []*DeferredNotificationInput
	Mentions              []*MentionInput
	ApproverUserID        uuid.UUID
	CreatedByUserID       uuid.UUID
	// Actor is the authenticated user creating the draft, resolved at the API
	// boundary. Carries the identity (email/name) the audit snapshot records;
	// CreatedByUserID stays the persisted author id. Always set on the public
	// path.
	Actor *User
}

type UpdateMaintenanceCmd struct {
	MaintID      uuid.UUID
	Title        *string
	Description  *string
	PlannedStart *time.Time
	Scope        *MaintenanceScope
	Impact       *MaintenanceImpact
	Resources    []uuid.UUID
	Steps        []*MaintenanceStepInput
	// NotifyTargets stays a plain slice with "empty = unchanged": a maintenance
	// must always keep at least one notify target, so zero targets is not a
	// legal state and there is nothing to express beyond "unchanged".
	NotifyTargets []*NotifyTargetInput // empty = unchanged
	// DeferredNotifications is tri-state, unlike NotifyTargets above: zero
	// reminders IS a legal state, so clearing must be distinguishable from
	// leaving them alone. nil = unchanged, empty slice = clear, non-empty =
	// replace.
	DeferredNotifications *[]*DeferredNotificationInput
	// Mentions is tri-state, like DeferredNotifications above: zero mentions IS
	// a legal state, so clearing must be distinguishable from leaving them
	// alone. nil = unchanged, empty slice = clear, non-empty = replace.
	Mentions       *[]*MentionInput
	ApproverUserID *uuid.UUID // nil = unchanged (no clear-to-null)
	// Actor is the authenticated user performing the mutation, resolved at the
	// API boundary for the audit snapshot. Always set on the public path.
	Actor *User
}
type StartMaintenanceCmd struct {
	MaintID uuid.UUID
	Actor   *User // audit actor, resolved at the API boundary
}

type CancelMaintenanceCmd struct {
	MaintID       uuid.UUID
	Reason        MaintenanceCancelReason
	ReasonComment string
	Actor         *User // audit actor, resolved at the API boundary
}

type CompleteMaintenanceCmd struct {
	MaintID uuid.UUID
	Actor   *User // audit actor, resolved at the API boundary
}

type ApproveMaintenanceCmd struct {
	MaintID               uuid.UUID
	ObservedMaintRevision int64
	ConflictSnapshot      ConflictsSnapshot
	// ActorUserID is the authenticated user performing the approve. Only the
	// user assigned as the maintenance approver may approve it.
	ActorUserID uuid.UUID
	// Actor is the same authenticated user, carrying the identity (email/name)
	// the audit snapshot records. ActorUserID stays the authz key.
	Actor *User
}

// --- Steps commands ---

type StartMaintenanceStepCmd struct {
	MaintID uuid.UUID
	StepID  uuid.UUID
	Actor   *User // audit actor, resolved at the API boundary
}

type CompleteMaintenanceStepCmd struct {
	MaintID uuid.UUID
	StepID  uuid.UUID
	Actor   *User // audit actor, resolved at the API boundary
}

type CancelMaintenanceStepCmd struct {
	MaintID uuid.UUID
	StepID  uuid.UUID
	Actor   *User // audit actor, resolved at the API boundary
}

// --- Conflicts commands ---

type ConflictQueryCmd struct {
	MaintID       uuid.UUID
	PlannedPeriod Period
	Scope         MaintenanceScope
	ResourceIDs   []uuid.UUID
}

// ActualConflictQueryCmd asks who actually ran alongside a maintenance that has
// already finished, matching real working windows rather than plans.
//
// ActualPeriod is the maintenance's own factual window and must be closed: the
// caller reaches this query only for a terminal maintenance that has one, and a
// maintenance that never ran takes the approval-snapshot path instead.
type ActualConflictQueryCmd struct {
	MaintID      uuid.UUID
	ActualPeriod Period
	Scope        MaintenanceScope
	ResourceIDs  []uuid.UUID
}

type SaveConflictsSnapshotCmd struct {
	MaintID          uuid.UUID
	ConflictSnapshot ConflictsSnapshot
}

type ConflictResourcesQueryCmd struct {
	ConflictedMaintIDs []uuid.UUID
}

// --- Resource commands ---

type CreateResourceCmd struct {
	Name        string
	Description string
	ExternalID  *string
	// CreatedByUserID is the author: the authenticated user from the access
	// token. The create path always requires an authenticated user.
	CreatedByUserID uuid.UUID
}

// UpdateResourceCmd describes a partial update of a resource. Each optional
// field, when non-nil, replaces the corresponding value; a nil field leaves it
// unchanged. ExternalID may be set to an empty string to clear it.
type UpdateResourceCmd struct {
	ID          uuid.UUID
	Name        *string
	Description *string
	ExternalID  *string
	// UpdatedByUserID is the editor: the authenticated user from the access
	// token. Recorded on every update.
	UpdatedByUserID uuid.UUID
}

// ListResourcesCmd describes a paginated resource listing request.
//
// Name, when non-empty, filters resources by a case-insensitive partial match
// on the name (LIKE %name%).
//
// IncludeArchived controls which statuses are returned:
//   - false: only active resources;
//   - true: both active and archived resources.
type ListResourcesCmd struct {
	Name            string
	Limit           int64
	Offset          int64
	IncludeArchived bool
}

// ListResourcesResult is a single page of resources plus the total count of
// resources matching the same filter (used by the API to expose pagination
// metadata).
type ListResourcesResult struct {
	Resources []*ResourceDetails
	Total     int64
}

// ListChannelsCmd describes a paginated notify-channel listing request.
//
// Name, when non-empty, filters channels by a case-insensitive partial match on
// the name (LIKE %name%). It is trimmed by the caller; the store treats an empty
// Name as "no filter".
//
// IncludeArchived widens the scope from active-only to active plus archived.
//
// Limit is applied verbatim: a zero Limit yields an empty page, not the whole
// catalog. Callers building this struct by hand must set it.
type ListChannelsCmd struct {
	Name            string
	Limit           int64
	Offset          int64
	IncludeArchived bool
}

// ListChannelsResult is a single page of channels plus the total count of
// channels matching the same filter, before LIMIT/OFFSET.
type ListChannelsResult struct {
	Channels []*NotifyChannel
	Total    int64
}

// --- Authorization commands ---

type ExchangeIDTokenCmd struct {
	Provider  AuthMethod
	IDToken   string
	ClientIP  string
	UserAgent string
	// TestRoles are extra roles requested via the dev-only X-Test-Roles header.
	// Filled only by the dev component of the API layer; the prod wiring is a
	// noop that never reads the header, so the field stays empty there.
	TestRoles []Role
}

type ConnectProviderCmd struct {
	UserID   uuid.UUID
	Provider AuthMethod
	IDToken  string
}

type DisconnectProviderCmd struct {
	UserID   uuid.UUID
	Provider AuthMethod
}

// --- Roles commands ---

// AssignRolesCmd adds one or more roles to a user, unioned with the roles they
// already hold.
type AssignRolesCmd struct {
	Actor  *User
	UserID uuid.UUID
	Roles  []Role
}

type RevokeRoleCmd struct {
	Actor  *User
	UserID uuid.UUID
	Role   Role
}

type ReplaceRolesCmd struct {
	Actor  *User
	UserID uuid.UUID
	Roles  []Role
}

// --- User management commands ---

// ListUsersCmd describes a paginated user listing request.
//
// Search, when non-empty, filters by a case-insensitive partial match on
// display name (name) and email (LIKE %search%). The messenger tags join that
// match only when SearchMessengerTags is set.
//
// SearchMessengerTags opts the telegram/slack tag columns into the Search
// match. It is set on the ADMIN listing path only. The picker leaves it false:
// its response carries no tags, so matching them there buys the caller nothing
// while turning the endpoint into an existence oracle — ask it whether
// "@someone" matches and it answers with that person's name and email. The
// picker is reachable by every role from guest up (maintenance.read), whereas
// the admin list is admin-only and returns the tags outright, so matching them
// there reveals nothing the caller cannot already read.
//
// Tags are stored verbatim ("@ruslan" and "ruslan" are different strings, see
// CanonicalTelegramTag), so the leading "@" and surrounding blanks are stripped
// off the query before it is matched against them: a tag pasted out of a
// complaint finds the same user as the bare name. The name and email branches
// keep matching the raw query, unchanged. The stored tag needs no stripping
// either — the pattern is unanchored, so a leading "@" in the column matches
// anyway. A query that trims away to nothing matches on name/email only, since
// an empty tag pattern would match every non-null tag.
//
// Roles, when non-empty, keeps only users that have ANY of these roles among
// their roles (OR semantics).
//
// IDs, when non-empty, restricts the result to users with these ids — the batch
// author-resolution path. Other filters still apply; the resolver sends no
// ExcludeBlocked so blocked/removed authors still resolve (they are labeled, not
// hidden).
//
// ExcludeBlocked, when true, hides users with blocked_at set. Assignment
// pickers set it so blocked users cannot be selected; the admin list leaves it
// false to keep showing blocked users (so they can be unblocked).
type ListUsersCmd struct {
	Search              string
	SearchMessengerTags bool
	Roles               []Role
	IDs                 []uuid.UUID
	Limit               int64
	Offset              int64
	ExcludeBlocked      bool
}

// ListUsersResult is a page of users plus the metadata the API layer needs to
// render last-admin lockout state.
type ListUsersResult struct {
	Users []*User
	Total int64
	// ActiveAdminCount is the number of non-blocked admins in the whole system,
	// used to compute is_last_admin per row.
	ActiveAdminCount int64
	// ProvidersByUser maps a user ID to its connected OAuth providers (provider
	// ASC). Users with no identities are absent. Fetched in one batch query to
	// avoid an N+1 over the page.
	ProvidersByUser map[uuid.UUID][]AuthMethod
}

// ListAssignableUsersQuery describes a request for users selectable in a
// maintenance assignment picker. It is served in-process by the auth module
// (owner of the users table); only active (non-blocked) users are returned.
type ListAssignableUsersQuery struct {
	Search string
	// Roles, when non-empty, keeps only users having ANY of these roles (OR).
	Roles  []Role
	Limit  int64
	Offset int64
}

// ListAssignableUsersResult is a page of active (non-blocked) users eligible for
// maintenance assignment, served in-process by the auth module.
type ListAssignableUsersResult struct {
	Users  []*User
	Total  int64
	Limit  int64
	Offset int64
}

type BlockUserCmd struct {
	Actor  *User
	UserID uuid.UUID
}

type UnblockUserCmd struct {
	Actor  *User
	UserID uuid.UUID
}

// UpdateUserTagsCmd is an admin's partial update of another user's messenger
// tags. Actor is the admin performing the edit (recorded in the audit trail),
// UserID the target being edited.
//
// Each tag is guarded by its own Set* flag rather than by the pointer being
// non-nil, because nil is a meaningful value here: it is the explicit reset to
// NULL. "Field absent from the request" and "field explicitly set to null" are
// therefore distinct inputs, and only the flag can tell them apart — the same
// convention as the self-service UpdatePreferencesCmd.
//
// Timezone is deliberately absent: it only affects how the owner sees their own
// interface, so an admin changing it would break someone's display without
// leaving any effect on notification delivery, which is what the admin is
// actually responsible for.
type UpdateUserTagsCmd struct {
	Actor  *User
	UserID uuid.UUID

	SetTelegramTag bool
	TelegramTag    *string

	SetSlackTag bool
	SlackTag    *string
}

// IsEmpty reports whether the command carries no tag field at all, i.e. the
// admin sent a patch that touches nothing.
func (c UpdateUserTagsCmd) IsEmpty() bool {
	return !c.SetTelegramTag && !c.SetSlackTag
}

// UpdatePreferencesCmd is a partial update of the caller's own preferences.
//
// Each field is guarded by its own Set* flag rather than by the pointer being
// non-nil, because nil is a meaningful value here: it is the explicit reset to
// NULL. "Field absent from the request" and "field explicitly set to null" are
// therefore distinct inputs, and only the flag can tell them apart.
//
// This is the opposite convention to entity.UpdateNotifyChannelCmd, where a nil
// field is simply left unchanged. The difference is deliberate: that command has
// no resettable field, this one has three.
type UpdatePreferencesCmd struct {
	SetTimezone bool
	Timezone    *string

	SetTelegramTag bool
	TelegramTag    *string

	SetSlackTag bool
	SlackTag    *string
}

// IsEmpty reports whether the command carries no field at all, i.e. the caller
// sent a patch that touches nothing.
func (c UpdatePreferencesCmd) IsEmpty() bool {
	return !c.SetTimezone && !c.SetTelegramTag && !c.SetSlackTag
}

// --- Audit log commands ---

type GetAuditLogsCmd struct {
	Filter *AuditFilter
	Limit  int64
	Offset int64
}
