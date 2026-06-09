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
	ApproverUserID        uuid.UUID
	CreatedByUserID       uuid.UUID
}

type UpdateMaintenanceCmd struct {
	MaintID               uuid.UUID
	Title                 *string
	Description           *string
	PlannedStart          *time.Time
	Scope                 *MaintenanceScope
	Impact                *MaintenanceImpact
	Resources             []uuid.UUID
	Steps                 []*MaintenanceStepInput
	NotifyTargets         []*NotifyTargetInput         // empty = unchanged
	DeferredNotifications []*DeferredNotificationInput // empty = unchanged
	ApproverUserID        *uuid.UUID                   // nil = unchanged (no clear-to-null)
}
type StartMaintenanceCmd struct {
	MaintID uuid.UUID
}

type CancelMaintenanceCmd struct {
	MaintID       uuid.UUID
	Reason        MaintenanceCancelReason
	ReasonComment string
}

type CompleteMaintenanceCmd struct {
	MaintID uuid.UUID
}

type ApproveMaintenanceCmd struct {
	MaintID               uuid.UUID
	ObservedMaintRevision int64
	ConflictSnapshot      ConflictsSnapshot
	// ActorUserID is the authenticated user performing the approve. Only the
	// user assigned as the maintenance approver may approve it.
	ActorUserID uuid.UUID
}

// --- Steps commands ---

type StartMaintenanceStepCmd struct {
	MaintID uuid.UUID
	StepID  uuid.UUID
}

type CompleteMaintenanceStepCmd struct {
	MaintID uuid.UUID
	StepID  uuid.UUID
}

type CancelMaintenanceStepCmd struct {
	MaintID uuid.UUID
	StepID  uuid.UUID
}

// --- Conflicts commands ---

type ConflictQueryCmd struct {
	MaintID       uuid.UUID
	PlannedPeriod Period
	Scope         MaintenanceScope
	ResourceIDs   []uuid.UUID
}

type SaveConflictsSnapshotCmd struct {
	MaintID          uuid.UUID
	ConflictSnapshot ConflictsSnapshot
}

type ConflictResourcesQueryCmd struct {
	MaintResourceIDs   []uuid.UUID
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

// --- Authorization commands ---

type GetAuthCodeURLCmd struct {
	Provider OAuthProvider
	State    string // State is an opaque, already-encoded value (typically signed via SignedStateCodec).
}

type HandleOAuthCallbackCmd struct {
	Provider     OAuthProvider
	CallbackCode string
	ClientIP     string
}

type ExchangeIDTokenCmd struct {
	Provider OAuthProvider
	IDToken  string
	ClientIP string
}

type ConnectProviderCmd struct {
	UserID   uuid.UUID
	Provider OAuthProvider
	IDToken  string
}

type DisconnectProviderCmd struct {
	UserID   uuid.UUID
	Provider OAuthProvider
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
// display name (name) OR email (LIKE %search%).
//
// Roles, when non-empty, keeps only users that have ANY of these roles among
// their roles (OR semantics).
//
// IDs, when non-empty, restricts the result to users with these ids — the batch
// author-resolution path (GET /api/v1/s2s/users?ids=...). Other filters still
// apply; the resolver sends no ExcludeBlocked so blocked/removed authors still
// resolve (they are labeled, not hidden).
//
// ExcludeBlocked, when true, hides users with blocked_at set. Assignment
// pickers set it so blocked users cannot be selected; the admin list leaves it
// false to keep showing blocked users (so they can be unblocked).
type ListUsersCmd struct {
	Search         string
	Roles          []Role
	IDs            []uuid.UUID
	Limit          int64
	Offset         int64
	ExcludeBlocked bool
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
	ProvidersByUser map[uuid.UUID][]OAuthProvider
}

// ListAssignableUsersQuery describes a request for users selectable in a
// maintenance assignment picker. It is served by the auth service (owner of the
// users table) over S2S; only active (non-blocked) users are returned.
type ListAssignableUsersQuery struct {
	Search string
	// Roles, when non-empty, keeps only users having ANY of these roles (OR).
	Roles  []Role
	Limit  int64
	Offset int64
}

// ListAssignableUsersResult is a page of active (non-blocked) users eligible for
// maintenance assignment, served by auth over S2S.
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

// --- Audit log commands ---

type GetAuditLogsCmd struct {
	Filter *AuditFilter
	Limit  int64
	Offset int64
}
