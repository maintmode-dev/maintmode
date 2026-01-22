package xtime

import (
	"time"

	"github.com/jackc/pgtype"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

func ToPgRange(p entity.Period) pgtype.Tstzrange {
	switch {
	case p.Start == nil && p.End == nil:
		// null period
		return pgtype.Tstzrange{}
	case p.Start != nil && p.End == nil:
		// open-ended period
		return pgRangeInLocation(pgtype.Tstzrange{
			Lower:     pgtype.Timestamptz{Time: lo.FromPtr(p.Start), Status: pgtype.Present},
			Upper:     pgtype.Timestamptz{},
			LowerType: pgtype.Inclusive,
			UpperType: pgtype.Unbounded,
			Status:    pgtype.Present,
		}, time.UTC)
	default:
		return pgRangeInLocation(pgtype.Tstzrange{
			Lower:     pgtype.Timestamptz{Time: lo.FromPtr(p.Start), Status: pgtype.Present},
			Upper:     pgtype.Timestamptz{Time: lo.FromPtr(p.End), Status: pgtype.Present},
			LowerType: pgtype.Inclusive,
			UpperType: pgtype.Exclusive,
			Status:    pgtype.Present,
		}, time.UTC)
	}
}

func FromPgRange(r pgtype.Tstzrange) entity.Period {
	period := entity.Period{}
	if r.Status != pgtype.Present {
		return period
	}

	if r.Lower.Status == pgtype.Present {
		period.Start = lo.ToPtr(r.Lower.Time.In(time.UTC))
	}
	if r.Upper.Status == pgtype.Present {
		period.End = lo.ToPtr(r.Upper.Time.In(time.UTC))
	}

	return period
}

func pgRangeInLocation(r pgtype.Tstzrange, loc *time.Location) pgtype.Tstzrange {
	newRange := r

	if r.Lower.Status == pgtype.Present {
		newRange.Lower.Time = r.Lower.Time.In(loc)
	}
	if r.Upper.Status == pgtype.Present {
		newRange.Upper.Time = r.Upper.Time.In(loc)
	}

	return newRange
}
