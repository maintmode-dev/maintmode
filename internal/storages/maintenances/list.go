package maintenances

import (
	"context"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/pkg/generated/postgres/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/postgres/public/table"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

type ListFilter struct {
	PeriodFrom *time.Time
	PeriodTo   *time.Time
}

func (f *ListFilter) toWhereExpr() postgres.BoolExpression {
	periodFrom := lo.Ternary(f.PeriodFrom != nil, lo.FromPtr(f.PeriodFrom), xtime.StartOfTheDay())
	periodTo := lo.Ternary(f.PeriodTo != nil, lo.FromPtr(f.PeriodTo), xtime.EndOfTheDay())

	expr := postgres.AND(
		table.Maintenances.PlannedPeriod.OVERLAP(
			postgres.TSTZ_RANGE(
				postgres.TimestampzT(periodFrom),
				postgres.TimestampzT(periodTo),
			),
		),
	)

	return expr
}

func (s *Store) List(ctx context.Context, filter *ListFilter, limit int64) ([]*entity.Maintenance, error) {
	ctx = xlog.WithOperation(ctx, "store.Maintenances.GetConflicts")

	stmt := table.Maintenances.
		SELECT(table.Maintenances.AllColumns).
		WHERE(filter.toWhereExpr()).
		ORDER_BY(table.Maintenances.CreatedAt.DESC()).
		LIMIT(limit)

	maints := make([]*model.Maintenances, 0)
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), &maints)
	if err != nil {
		return nil, err
	}

	return lo.Map(maints, func(item *model.Maintenances, _ int) *entity.Maintenance {
		return fromDBMaintenance(item)
	}), nil
}
