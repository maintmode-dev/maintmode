package conflicts

import (
	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

type Conflict struct {
	MaintID       uuid.UUID
	Scope         string
	OverlapPeriod pgtype.Tstzrange
}

func fromDBConflict(c *Conflict) *entity.Conflict {
	period := xtime.FromPgRange(c.OverlapPeriod)
	return &entity.Conflict{
		MaintenanceID: c.MaintID,
		Scope:         entity.MaintenanceScope(c.Scope),
		OverlapStart:  lo.FromPtr(period.Start),
		OverlapEnd:    lo.FromPtr(period.End),
	}
}

func uuidsToPgUUID(resourceIDs []uuid.UUID) []postgres.StringExpression {
	return lo.Map(resourceIDs, func(item uuid.UUID, _ int) postgres.StringExpression {
		return postgres.UUID(item)
	})
}
