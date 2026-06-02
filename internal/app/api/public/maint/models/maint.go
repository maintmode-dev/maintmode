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
	Order               int32  `json:"order"`
	Description         string `json:"description"`
	RollbackDescription string `json:"rollback_description"`
	Duration            string `json:"duration"`
}

type MaintenanceCancelReason string

const (
	MaintenanceCancelReasonConflict         MaintenanceCancelReason = "conflict"
	MaintenanceCancelReasonIncident         MaintenanceCancelReason = "incident"
	MaintenanceCancelReasonBusinessDecision MaintenanceCancelReason = "business_decision"
	MaintenanceCancelReasonRescheduled      MaintenanceCancelReason = "rescheduled"
	MaintenanceCancelReasonMistake          MaintenanceCancelReason = "mistake"
)

type NotifyTargets struct {
	ChannelIDs []string `json:"channel_ids" example:"[slack:C0123456, tg:-10013123]"`
}

// DeferredNotification is one entry of the deferred_notifications array: a
// reminder to fire at fire_at. The reminder is delivered to the maintenance's
// notify targets and its text is rendered server-side, so only the schedule is
// part of the contract.
type DeferredNotification struct {
	FireAt time.Time `json:"fire_at" format:"date-time"`
}

type Maintenance struct {
	ID                    uuid.UUID               `json:"id" format:"uuid"`
	Title                 string                  `json:"title"`
	Description           string                  `json:"description"`
	PlannedPeriod         Period                  `json:"planned_period"`
	ActualPeriod          *Period                 `json:"actual_period"`
	Resources             []*ResourceRef          `json:"resources"`
	Scope                 MaintenanceScope        `json:"scope"`
	Impact                MaintenanceImpact       `json:"impact"`
	Status                string                  `json:"status"`
	CancelReason          MaintenanceCancelReason `json:"cancel_reason"`
	CancelReasonComment   string                  `json:"cancel_reason_comment"`
	CreatedAt             time.Time               `json:"created_at" format:"date-time"`
	UpdatedAt             *time.Time              `json:"updated_at" format:"date-time"`
	Steps                 []*MaintenanceStep      `json:"steps"`
	NotifyTargets         *NotifyTargets          `json:"notify_targets"`
	DeferredNotifications []*DeferredNotification `json:"deferred_notifications"`
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
	CreatedAt             time.Time               `json:"created_at" format:"date-time"`
	Steps                 []*MaintenanceStep      `json:"steps"`
	NotifyTargets         *NotifyTargets          `json:"notify_targets"`
	DeferredNotifications []*DeferredNotification `json:"deferred_notifications"`
}

type UpdateDraftMaintRequest struct {
	Title                 string                  `json:"title" example:"DB migration"`
	Description           string                  `json:"description" example:"PostgreSQL major upgrade"`
	PlannedStart          time.Time               `json:"planned_start" format:"date-time"`
	Scope                 MaintenanceScope        `json:"scope"`
	Impact                MaintenanceImpact       `json:"impact"`
	Resources             []*ResourceRef          `json:"resources"`
	Steps                 []*MaintenanceStepInput `json:"steps"`
	NotifyTargets         *NotifyTargets          `json:"notify_targets"`
	DeferredNotifications []*DeferredNotification `json:"deferred_notifications"`
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
