package entity

import (
	"time"

	"github.com/google/uuid"
)

type MaintenanceStatus string

const (
	MaintenanceStatusDraft      MaintenanceStatus = "draft"
	MaintenanceStatusPlanned    MaintenanceStatus = "planned"
	MaintenanceStatusInProgress MaintenanceStatus = "in_progress"
	MaintenanceStatusCancelled  MaintenanceStatus = "canceled"
	MaintenanceStatusCompleted  MaintenanceStatus = "completed"
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

type MaintenanceCancelReason string

const (
	MaintenanceCancelReasonConflict         MaintenanceCancelReason = "conflict"
	MaintenanceCancelReasonIncident         MaintenanceCancelReason = "incident"
	MaintenanceCancelReasonBusinessDecision MaintenanceCancelReason = "business_decision"
	MaintenanceCancelReasonRescheduled      MaintenanceCancelReason = "rescheduled"
	MaintenanceCancelReasonMistake          MaintenanceCancelReason = "mistake"
)

type Maintenance struct {
	ID                  uuid.UUID
	Title               string
	Description         string
	PlannedPeriod       Period
	ActualPeriod        *Period
	Resources           []*Resource
	Scope               MaintenanceScope
	Impact              MaintenanceImpact
	Status              MaintenanceStatus
	CancelReason        MaintenanceCancelReason
	CancelReasonComment string
	CreatedAt           time.Time
	UpdatedAt           *time.Time
}

func (m *Maintenance) Revision() int64 {
	if m.UpdatedAt != nil {
		return m.UpdatedAt.UnixMicro()
	}
	return m.CreatedAt.UnixMicro()
}
