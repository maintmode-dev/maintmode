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

type CreateMaintenanceCmd struct {
	Title         string
	Description   string
	PlannedPeriod Period
	Scope         MaintenanceScope
	Impact        MaintenanceImpact
	Resources     []*Resource
	Steps         []*MaintenanceStepInput
}

type UpdateMaintenanceCmd struct {
	MaintID      uuid.UUID
	Title        *string
	Description  *string
	PlannedStart *time.Time
	Scope        *MaintenanceScope
	Impact       *MaintenanceImpact
	Resources    []*Resource
	Steps        []*MaintenanceStepInput
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
