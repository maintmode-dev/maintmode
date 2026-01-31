package calendardto

import (
	"time"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/entity"
)

type MaintenanceResource struct {
	ID   uuid.UUID
	Type entity.ResourceType
	Name string
}

type Maintenance struct {
	ID                  uuid.UUID
	Title               string
	Description         string
	PlannedPeriod       entity.Period
	ActualPeriod        *entity.Period
	Resources           []*MaintenanceResource
	Scope               entity.MaintenanceScope
	Impact              entity.MaintenanceImpact
	Status              entity.MaintenanceStatus
	CancelReason        entity.MaintenanceCancelReason
	CancelReasonComment string
	CreatedAt           time.Time
	UpdatedAt           *time.Time
	Revision            int64
}

type MaintenancesMeta struct {
	Count     int64
	Truncated bool
}
