package calendardto

import (
	"time"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/entity"
)

type GetMaintsFilter struct {
	PeriodFrom  time.Time
	PeriodTo    time.Time
	Statuses    []entity.MaintenanceStatus
	ResourceIDs []uuid.UUID
}

type ConflictQueryCmd entity.ConflictQueryCmd
