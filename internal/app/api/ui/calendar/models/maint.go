package uimodels

import (
	"time"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/entity"
)

// MaintenanceStatus represents maintenance status.
// @Enums(draft, planned, in_progress, completed, canceled)
type MaintenanceStatus = entity.MaintenanceStatus

type MaintenanceViewResource struct {
	ID   uuid.UUID `json:"id" format:"uuid"`
	Name string    `json:"name"`
}

// NotifyTargetView is one notify-channel chip of the maintenance view: the
// catalog channel's uuid and human name plus the delivery transport, resolved
// from the channel catalog (targets reference channels by FK).
type NotifyTargetView struct {
	ID        uuid.UUID `json:"id" format:"uuid"`
	Name      string    `json:"name" example:"Platform alerts"`
	Transport string    `json:"transport" example:"slack"`
}

// DeferredNotificationView is one scheduled reminder of the maintenance view.
// Scheduled is false while the maintenance is a draft and becomes true once the
// reminder has been enqueued on approve.
type DeferredNotificationView struct {
	ID        uuid.UUID `json:"id" format:"uuid"`
	FireAt    time.Time `json:"fire_at" format:"date-time"`
	Scheduled bool      `json:"scheduled"`
}

type MaintenanceStep struct {
	ID                  uuid.UUID `json:"id" format:"uuid"`
	Order               int32     `json:"order"`
	Description         string    `json:"description"`
	RollbackDescription string    `json:"rollback_description"`
	Status              string    `json:"status"`
	Duration            string    `json:"duration"`
	PlannedTimeStart    time.Time `json:"planned_time_start" format:"date-time"`
	PlannedTimeEnd      time.Time `json:"planned_time_end" format:"date-time"`
}

// UserSummary is the author/user view attached to UI read responses: {id,
// display_name, email}. A nil *UserSummary serializes as null. When the author
// could not be resolved from auth, display_name is "Unknown user" (the id is
// still carried) rather than null.
type UserSummary struct {
	ID          uuid.UUID `json:"id" format:"uuid"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
}

type MaintenanceView struct {
	ID                  uuid.UUID                  `json:"id" format:"uuid"`
	Title               string                     `json:"title"`
	Description         string                     `json:"description"`
	PlannedTimeStart    time.Time                  `json:"planned_time_start" format:"date-time"`
	PlannedTimeEnd      time.Time                  `json:"planned_time_end" format:"date-time"`
	ActualTimeStart     *time.Time                 `json:"actual_time_start,omitempty" format:"date-time"`
	ActualTimeEnd       *time.Time                 `json:"actual_time_end,omitempty" format:"date-time"`
	Resources           []*MaintenanceViewResource `json:"resources"`
	Scope               string                     `json:"scope"`
	Impact              string                     `json:"impact"`
	Status              MaintenanceStatus          `json:"status"`
	CancelReason        string                     `json:"cancel_reason"`
	CancelReasonComment string                     `json:"cancel_reason_comment"`
	CreatedAt           time.Time                  `json:"created_at" format:"date-time"`
	UpdatedAt           *time.Time                 `json:"updated_at" format:"date-time"`
	Revision            int64                      `json:"revision"`
	// CreatedBy is the author resolved from auth; always present (degrades to the
	// "Unknown user" label when unresolvable). Approver is the assigned approver,
	// or null when none is set (e.g. a draft).
	CreatedBy *UserSummary       `json:"created_by"`
	Approver  *UserSummary       `json:"approver"`
	Steps     []*MaintenanceStep `json:"steps"`
	// NotifyTargets lists the maintenance's notify channels resolved from the
	// catalog, for the read-only Notify channels section (transport glyph + name).
	NotifyTargets []*NotifyTargetView `json:"notify_targets"`
	// DeferredNotifications lists the maintenance's reminders ordered by fire_at,
	// so the edit screen can hydrate the already saved schedule. Always an array;
	// empty when no reminders are set.
	DeferredNotifications []*DeferredNotificationView `json:"deferred_notifications"`
}

type ConflictView struct {
	MaintenanceID uuid.UUID                  `json:"maintenance_id" format:"uuid"`
	Title         string                     `json:"title"`
	OverlapStart  time.Time                  `json:"overlap_start" format:"date-time"`
	OverlapEnd    time.Time                  `json:"overlap_end" format:"date-time"`
	Scope         string                     `json:"scope"`
	Resources     []*MaintenanceViewResource `json:"resources"`
}

type MaintenanceActions struct {
	CanEdit     bool `json:"can_edit"`
	CanApprove  bool `json:"can_approve"`
	CanStart    bool `json:"can_start"`
	CanCancel   bool `json:"can_cancel"`
	CanComplete bool `json:"can_complete"`
}

type MaintenanceViewResponse struct {
	Maintenance *MaintenanceView    `json:"maintenance"`
	Conflicts   []*ConflictView     `json:"conflicts"`
	Actions     *MaintenanceActions `json:"actions"`
}
