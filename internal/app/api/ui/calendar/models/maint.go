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
	Steps               []*MaintenanceStep         `json:"steps"`
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
