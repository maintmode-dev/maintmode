package audit

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

type countByActionRow struct {
	Action string `alias:"action.name"`
	Count  int64  `alias:"action.count"`
}

// CountByAction returns entry counts grouped by action under the filter.
// Category aggregation happens in the service layer.
func (s *Store) CountByAction(ctx context.Context, f *entity.AuditFilter) (map[entity.AuditAction]int64, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Audit.CountByAction")
	defer span.End()

	stmt := table.AuditLog.
		SELECT(
			table.AuditLog.Action.AS("action.name"),
			postgres.COUNT(postgres.STAR).AS("action.count"),
		).
		WHERE(filterToWhereExp(f)).
		GROUP_BY(table.AuditLog.Action)

	rows := make([]*countByActionRow, 0)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &rows); err != nil {
		return nil, err
	}

	return lo.SliceToMap(rows, func(item *countByActionRow) (entity.AuditAction, int64) {
		return entity.AuditAction(item.Action), item.Count
	}), nil
}
