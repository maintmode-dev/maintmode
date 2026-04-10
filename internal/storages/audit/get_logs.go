package audit

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) GetLogs(ctx context.Context, f *entity.AuditFilter, limit int64) ([]*entity.AuditEntry, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Audit.GetLogs")
	defer span.End()

	stmt := table.AuditLog.
		SELECT(table.AuditLog.AllColumns).
		WHERE(filterToWhereExp(f)).
		ORDER_BY(table.AuditLog.CreatedAt.DESC()).
		LIMIT(limit)

	logs := make([]*model.AuditLog, 0, limit)
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), &logs)
	if err != nil {
		return nil, err
	}

	return lo.Map(logs, func(item *model.AuditLog, _ int) *entity.AuditEntry {
		return fromDBEntry(item)
	}), nil
}

func filterToWhereExp(f *entity.AuditFilter) postgres.BoolExpression {
	if f == nil {
		return postgres.Bool(true)
	}

	return nil
}
