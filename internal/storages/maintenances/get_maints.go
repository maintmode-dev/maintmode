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
	ctx, span := xlog.WithOperationSpan(ctx, "store.Maintenances.GetMaints.CalendarView")
	defer span.End()

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

	// The upper default is anchored to periodFrom, not to today. Anchoring it to
	// today made the two defaults independent, and a caller who supplied only
	// period_from -- tomorrow, say -- got a range whose lower bound was above
	// its upper. Postgres rejects that outright (22000, "range lower bound must
	// be less than or equal to range upper bound"), so the request failed with a
	// 500 where the honest answer is "the rest of that day".
	//
	// It also made the query time-dependent in a way nothing declared: the same
	// call answered differently before and after midnight UTC. TestList/no_overlap
	// asks about a maintenance starting two hours out and was red for the last
	// two hours of every UTC day for exactly this reason.
	periodTo := lo.Ternary(f.PeriodTo.IsZero(), xtime.EndOfTheDay(periodFrom), f.PeriodTo)

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

	if len(f.ChannelIDs) > 0 {
		// notify targets reference catalog channels by uuid (FK), so the filter
		// matches the stored channel_id directly. Archived channels are matched
		// on purpose: the channel's Related section is a history view and must
		// still surface past maintenances.
		expr = expr.AND(
			table.Maintenances.ID.IN(
				table.MaintenanceNotifyTargets.
					SELECT(table.MaintenanceNotifyTargets.MaintenanceID).
					WHERE(table.MaintenanceNotifyTargets.ChannelID.EQ(
						postgres.ANY(postgres.ARRAY(uuidsToPgUUID(f.ChannelIDs)...)),
					)),
			),
		)
	}

	return expr
}
