package maintenances

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/calendardto"

	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func (s *Store) GetMaints(ctx context.Context, filter *calendardto.GetMaintsFilter, limit int64) ([]*entity.Maintenance, bool, error) {
	ctx = xlog.WithOperation(ctx, "store.Maintenances.CalendarView")

	stmt := table.Maintenances.
		SELECT(table.Maintenances.AllColumns).
		WHERE(filterToWhereExpr(filter)).
		ORDER_BY(table.Maintenances.PlannedPeriod.DESC(), table.Maintenances.ID.DESC()).
		LIMIT(limit + 1)

	maints := make([]*model.Maintenances, 0, limit+1)
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), &maints)
	if err != nil {
		return nil, false, err
	}

	// truncated means that there are more events than requested
	truncated := false
	if len(maints) > int(limit) {
		truncated = true
		maints = maints[:limit]
	}

	return lo.Map(maints, func(item *model.Maintenances, _ int) *entity.Maintenance {
		return fromDBMaintenance(item)
	}), truncated, nil
}

func filterToWhereExpr(f *calendardto.GetMaintsFilter) postgres.BoolExpression {
	periodFrom := lo.Ternary(f.PeriodFrom.IsZero(), xtime.StartOfTheCurrentDay(), f.PeriodFrom)
	periodTo := lo.Ternary(f.PeriodTo.IsZero(), xtime.EndOfTheCurrentDay(), f.PeriodTo)

	expr := postgres.AND(
		table.Maintenances.PlannedPeriod.OVERLAP(
			postgres.TSTZ_RANGE(
				postgres.TimestampzT(periodFrom),
				postgres.TimestampzT(periodTo),
			),
		),
	)

	if len(f.Statuses) > 0 {
		expr = expr.AND(table.Maintenances.Status.IN(
			lo.Map(f.Statuses, func(item entity.MaintenanceStatus, _ int) postgres.Expression {
				return postgres.String(string(item))
			})...,
		))
	}

	if len(f.ResourceIDs) > 0 {
		expr = expr.AND(
			table.Maintenances.ID.IN(
				table.MaintenanceResources.
					SELECT(table.MaintenanceResources.MaintenanceID).
					WHERE(table.MaintenanceResources.ResourceID.EQ(
						postgres.ANY(postgres.ARRAY(uuidsToPgUUID(f.ResourceIDs)...)),
					)),
			),
		)
	}

	return expr
}
