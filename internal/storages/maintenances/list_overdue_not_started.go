package maintenances

import (
	"context"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// ListOverdueNotStarted returns maintenances that never started in time: those
// still in a not-started status (draft or planned) whose planned start (the lower
// bound of planned_period) is strictly before startedBefore. It is the query
// behind the auto-cancel job.
//
// Both draft and planned count as "not started": planned was approved but no one
// hit start, and draft was never even approved — neither can transition to
// in_progress on its own. in_progress / completed / canceled are excluded because
// they have already run (or been decided on).
//
// Unlike GetMaints (the calendar view, which uses range OVERLAP and defaults to
// the current day), this matches purely on the planned-start instant, so it can
// look arbitrarily far into the past. Results are ordered oldest-start-first so a
// bounded batch always drains the most overdue maintenances first. Child
// collections (steps, resources, notify targets) are not loaded — the caller only
// needs the id and status to cancel.
func (s *Store) ListOverdueNotStarted(ctx context.Context, startedBefore time.Time, limit int64) ([]*entity.Maintenance, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Maintenances.ListOverdueNotStarted")
	defer span.End()

	stmt := table.Maintenances.
		SELECT(table.Maintenances.AllColumns).
		WHERE(
			table.Maintenances.Status.IN(
				postgres.String(string(entity.MaintenanceStatusDraft)),
				postgres.String(string(entity.MaintenanceStatusPlanned)),
			).
				AND(
					postgres.LOWER_BOUND(table.Maintenances.PlannedPeriod).
						LT(postgres.TimestampzT(startedBefore)),
				),
		).
		ORDER_BY(postgres.LOWER_BOUND(table.Maintenances.PlannedPeriod).ASC()).
		LIMIT(limit)

	maints := make([]*model.Maintenances, 0, limit)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &maints); err != nil {
		return nil, err
	}

	return lo.Map(maints, func(item *model.Maintenances, _ int) *entity.Maintenance {
		return fromDBMaintenance(item)
	}), nil
}
