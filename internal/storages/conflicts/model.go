package conflicts

import (
	"github.com/google/uuid"
	"github.com/jackc/pgtype"
)

type Conflict struct {
	MaintID       uuid.UUID
	Title         string
	Scope         string
	OverlapPeriod pgtype.Tstzrange
}
