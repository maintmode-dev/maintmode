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

func (s *Store) GetLogs(ctx context.Context, cmd *entity.GetAuditLogsCmd) ([]*entity.AuditEntry, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Audit.GetLogs")
	defer span.End()

	stmt := table.AuditLog.
		SELECT(table.AuditLog.AllColumns).
		WHERE(filterToWhereExp(cmd.Filter)).
		ORDER_BY(table.AuditLog.CreatedAt.DESC()).
		LIMIT(cmd.Limit).
		OFFSET(cmd.Offset)

	logs := make([]*model.AuditLog, 0, cmd.Limit)
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), &logs)
	if err != nil {
		return nil, err
	}

	return lo.Map(logs, func(item *model.AuditLog, _ int) *entity.AuditEntry {
		return fromDBEntry(item)
	}), nil
}

func filterToWhereExp(f *entity.AuditFilter) postgres.BoolExpression {
	cond := postgres.Bool(true)
	if f == nil {
		return cond
	}

	if f.Action != nil {
		action := lo.FromPtr(f.Action)
		cond = cond.AND(table.AuditLog.Action.EQ(postgres.String(string(action))))
	}
	if f.Actor != nil {
		cond = cond.AND(table.AuditLog.Actor.EQ(postgres.String(lo.FromPtr(f.Actor))))
	}
	if f.CreatedFrom != nil {
		cond = cond.AND(table.AuditLog.CreatedAt.GT_EQ(postgres.TimestampzT(lo.FromPtr(f.CreatedFrom))))
	}
	if f.CreatedTo != nil {
		cond = cond.AND(table.AuditLog.CreatedAt.LT_EQ(postgres.TimestampzT(lo.FromPtr(f.CreatedTo))))
	}

	return cond
}
