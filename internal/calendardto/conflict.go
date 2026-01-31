package calendardto

import (
	"time"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/entity"
)

type Conflict struct {
	MaintenanceID uuid.UUID
	Title         string
	OverlapStart  time.Time
	OverlapEnd    time.Time
	Scope         entity.MaintenanceScope
	Resources     []*MaintenanceResource
}
