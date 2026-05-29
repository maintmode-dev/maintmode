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
	ChannelID string
}

type CreateMaintenanceCmd struct {
	Title         string
	Description   string
	PlannedPeriod Period
	Scope         MaintenanceScope
	Impact        MaintenanceImpact
	Resources     []uuid.UUID
	Steps         []*MaintenanceStepInput
	NotifyTargets []*NotifyTargetInput
}

type UpdateMaintenanceCmd struct {
	MaintID       uuid.UUID
	Title         *string
	Description   *string
	PlannedStart  *time.Time
	Scope         *MaintenanceScope
	Impact        *MaintenanceImpact
	Resources     []uuid.UUID
	Steps         []*MaintenanceStepInput
	NotifyTargets []*NotifyTargetInput // nil = unchanged
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
}

// UpdateResourceCmd describes a partial update of a resource. Each optional
// field, when non-nil, replaces the corresponding value; a nil field leaves it
// unchanged. ExternalID may be set to an empty string to clear it.
type UpdateResourceCmd struct {
	ID          uuid.UUID
	Name        *string
	Description *string
	ExternalID  *string
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

// --- Roles commands ---

type AssignRoleCmd struct {
	Actor  *User
	UserID uuid.UUID
	Role   Role
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

// --- Audit log commands ---

type GetAuditLogsCmd struct {
	Filter *AuditFilter
	Limit  int64
	Offset int64
}
