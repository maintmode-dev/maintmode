package apimodels

import (
	"time"

	"github.com/google/uuid"
)

type MaintenanceScope string

const (
	MaintenanceScopeGlobal    MaintenanceScope = "global"
	MaintenanceScopeResources MaintenanceScope = "resource"
)

type MaintenanceImpact string

const (
	MaintenanceImpactNone    MaintenanceImpact = "none"
	MaintenanceImpactPartial MaintenanceImpact = "partial_outage"
	MaintenanceImpactFull    MaintenanceImpact = "full_outage"
)

type MaintenanceStepStatus string

const (
	MaintenanceStepStatusUnknown   MaintenanceStepStatus = "unknown"
	MaintenanceStepStatusPlanned   MaintenanceStepStatus = "planned"
	MaintenanceStepStatusStarted   MaintenanceStepStatus = "started"
	MaintenanceStepStatusCompleted MaintenanceStepStatus = "completed"
	MaintenanceStepStatusCanceled  MaintenanceStepStatus = "canceled"
)

type MaintenanceStep struct {
	ID                  uuid.UUID             `json:"id" format:"uuid"`
	Order               int32                 `json:"order"`
	Description         string                `json:"description"`
	RollbackDescription string                `json:"rollback_description"`
	DurationMinutes     int64                 `json:"duration"`
	Status              MaintenanceStepStatus `json:"status"`
}

type MaintenanceStepInput struct {
	Order               int32  `json:"order" example:"1"`
	Description         string `json:"description"`
	RollbackDescription string `json:"rollback_description"`
	Duration            string `json:"duration" example:"1h30m"`
}

type MaintenanceCancelReason string

const (
	MaintenanceCancelReasonConflict         MaintenanceCancelReason = "conflict"
	MaintenanceCancelReasonIncident         MaintenanceCancelReason = "incident"
	MaintenanceCancelReasonBusinessDecision MaintenanceCancelReason = "business_decision"
	MaintenanceCancelReasonRescheduled      MaintenanceCancelReason = "rescheduled"
	MaintenanceCancelReasonMistake          MaintenanceCancelReason = "mistake"
)

// NotifyTargets carries catalog channel uuids: requests subscribe a
// maintenance to channels by id, and create/update responses echo the same
// ids back.
type NotifyTargets struct {
	ChannelIDs []string `json:"channel_ids" example:"0197a3c1-7a2b-7c3d-9e4f-5a6b7c8d9e0f"`
}

// NotifyTargetView is one notify-channel chip of the maintenance detail view:
// the catalog channel's uuid and human name plus the delivery transport,
// resolved from the channel catalog (targets reference channels by FK).
type NotifyTargetView struct {
	ID        uuid.UUID `json:"id" format:"uuid"`
	Name      string    `json:"name" example:"Platform alerts"`
	Transport string    `json:"transport" example:"slack"`
}

// DeferredNotification is one entry of the deferred_notifications array: a
// reminder to fire at fire_at. The reminder is delivered to the maintenance's
// notify targets and its text is rendered server-side, so only the schedule is
// part of the contract.
type DeferredNotification struct {
	FireAt time.Time `json:"fire_at" format:"date-time"`
}

// Mention is one entry of the create/update contract's mentions array: a
// MaintMode user tagged alongside the owner in the maintenance notification.
// An object rather than a bare uuid so the contract can gain keys without a
// breaking change.
type Mention struct {
	UserID uuid.UUID `json:"user_id" format:"uuid"`
}

// MentionView is one mention of a maintenance read view: who was tagged, with
// the display name resolved from auth. It deliberately carries no messenger-tag
// information — the detail view shows who was mentioned, not who has a
// messenger configured, and this endpoint is readable by guests.
type MentionView struct {
	UserID      uuid.UUID `json:"user_id" format:"uuid"`
	DisplayName string    `json:"display_name"`
}

// UserSummary is a privacy-safe view of a user (e.g. a maintenance author or
// assigned approver) exposed in API responses. It carries only what the UI
// needs to render a name, never internal fields. A nil *UserSummary is
// serialized as null (unknown/unassigned), so clients must handle the null case.
type UserSummary struct {
	ID          uuid.UUID `json:"id" format:"uuid"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
}

type Maintenance struct {
	ID                  uuid.UUID               `json:"id" format:"uuid"`
	Title               string                  `json:"title"`
	Description         string                  `json:"description"`
	PlannedPeriod       Period                  `json:"planned_period"`
	ActualPeriod        *Period                 `json:"actual_period"`
	Resources           []*ResourceRef          `json:"resources"`
	Scope               MaintenanceScope        `json:"scope"`
	Impact              MaintenanceImpact       `json:"impact"`
	Status              string                  `json:"status"`
	CancelReason        MaintenanceCancelReason `json:"cancel_reason"`
	CancelReasonComment string                  `json:"cancel_reason_comment"`
	CreatedAt           time.Time               `json:"created_at" format:"date-time"`
	UpdatedAt           *time.Time              `json:"updated_at" format:"date-time"`
	// CreatedBy is the maintenance author resolved from the auth service. Always
	// present on read (carries the author id); display_name is "Unknown user"
	// when the profile could not be resolved (auth down or user removed).
	CreatedBy *UserSummary `json:"created_by"`
	// Approver is the assigned approver resolved from the auth service, or null
	// when no approver is set (e.g. a draft). When set but unresolvable it
	// degrades to the "Unknown user" label like CreatedBy.
	Approver *UserSummary       `json:"approver"`
	Steps    []*MaintenanceStep `json:"steps"`
	// NotifyTargets lists the maintenance's notify channels resolved from the
	// catalog for read-only rendering. Unlike the create/update requests (which
	// take channel uuids via channel_ids), the detail view carries the full chip
	// data: id + name + transport.
	NotifyTargets         []*NotifyTargetView     `json:"notify_targets"`
	DeferredNotifications []*DeferredNotification `json:"deferred_notifications"`
	// Mentions lists the users tagged alongside the owner in this maintenance's
	// notifications, with display names resolved from auth.
	Mentions []*MentionView `json:"mentions"`
}

type CreateDraftMaintRequest struct {
	Title                 string                  `json:"title" example:"DB migration"`
	Description           string                  `json:"description" example:"PostgreSQL major upgrade"`
	PlannedStart          time.Time               `json:"planned_start" format:"date-time"`
	Scope                 MaintenanceScope        `json:"scope"`
	Impact                MaintenanceImpact       `json:"impact"`
	Resources             []*ResourceRef          `json:"resources"`
	Steps                 []*MaintenanceStepInput `json:"steps"`
	NotifyTargets         *NotifyTargets          `json:"notify_targets"`
	DeferredNotifications []*DeferredNotification `json:"deferred_notifications"`
	// Mentions tags additional MaintMode users in this maintenance's
	// notifications. Optional; omit it or send an empty array to tag no one.
	Mentions       []*Mention `json:"mentions"`
	ApproverUserID uuid.UUID  `json:"approver_user_id" format:"uuid"`
}

type CreateDraftMaintResponse struct {
	ID                    uuid.UUID               `json:"id" format:"uuid"`
	Title                 string                  `json:"title"`
	Description           string                  `json:"description"`
	PlannedPeriod         Period                  `json:"planned_period"`
	Resources             []*ResourceRef          `json:"resources"`
	Scope                 string                  `json:"scope"`
	Impact                string                  `json:"impact"`
	Status                string                  `json:"status"`
	CreatedBy             *UserSummary            `json:"created_by"`
	ApproverUserID        *UserSummary            `json:"approver_user"`
	CreatedAt             time.Time               `json:"created_at" format:"date-time"`
	Steps                 []*MaintenanceStep      `json:"steps"`
	NotifyTargets         *NotifyTargets          `json:"notify_targets"`
	DeferredNotifications []*DeferredNotification `json:"deferred_notifications"`
	// Mentions echoes back the tagged user ids so the form can read what it just
	// wrote. This response is hand-assembled (not built by ToAPIMaintenance), so
	// the field has to be carried here explicitly.
	Mentions []*Mention `json:"mentions"`
}

type UpdateDraftMaintRequest struct {
	Title         string                  `json:"title" example:"DB migration"`
	Description   string                  `json:"description" example:"PostgreSQL major upgrade"`
	PlannedStart  time.Time               `json:"planned_start" format:"date-time"`
	Scope         MaintenanceScope        `json:"scope"`
	Impact        MaintenanceImpact       `json:"impact"`
	Resources     []*ResourceRef          `json:"resources"`
	Steps         []*MaintenanceStepInput `json:"steps"`
	NotifyTargets *NotifyTargets          `json:"notify_targets"`
	// DeferredNotifications is tri-state on update: a missing field or null
	// leaves the reminders unchanged, an empty array clears them, and a
	// non-empty array replaces them. The distinction is carried by the pointer,
	// so it must stay a pointer all the way down to the service gate.
	DeferredNotifications *[]*DeferredNotification `json:"deferred_notifications"`
	// Mentions is tri-state on update, like DeferredNotifications above: a
	// missing field or null leaves the mentions unchanged, an empty array clears
	// them, and a non-empty array replaces them. The distinction is carried by
	// the pointer, so it must stay a pointer all the way down to the service gate.
	Mentions       *[]*Mention `json:"mentions"`
	ApproverUserID *uuid.UUID  `json:"approver_user_id" format:"uuid"`
}

type CancelMaintRequest struct {
	Reason  MaintenanceCancelReason `json:"reason"`
	Comment string                  `json:"comment"`
}

type Conflict struct {
	MaintenanceID uuid.UUID        `json:"maintenance_id" format:"uuid"`
	OverlapStart  time.Time        `json:"overlap_start" format:"date-time"`
	OverlapEnd    time.Time        `json:"overlap_end" format:"date-time"`
	Scope         MaintenanceScope `json:"scope"`
	Resources     []*ResourceRef   `json:"resources"`
}

type ApproveDraftMaintRequest struct {
	ObservedMaintRevision int64       `json:"observed_maint_revision"`
	ConflictsSnapshot     []*Conflict `json:"conflicts_snapshot"`
}
