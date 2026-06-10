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

// GetLogs returns a page of audit log entries ordered by created_at DESC,
// together with the total number of entries matching the same filter.
func (s *Store) GetLogs(ctx context.Context, cmd *entity.GetAuditLogsCmd) ([]*entity.AuditEntry, int64, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Audit.GetLogs")
	defer span.End()

	where := filterToWhereExp(cmd.Filter)

	total, err := s.countLogs(ctx, where)
	if err != nil {
		return nil, 0, err
	}

	stmt := table.AuditLog.
		SELECT(table.AuditLog.AllColumns).
		WHERE(where).
		ORDER_BY(table.AuditLog.CreatedAt.DESC()).
		LIMIT(cmd.Limit).
		OFFSET(cmd.Offset)

	logs := make([]*model.AuditLog, 0, cmd.Limit)
	err = stmt.QueryContext(ctx, s.db.Executor(ctx), &logs)
	if err != nil {
		return nil, 0, err
	}

	return lo.Map(logs, func(item *model.AuditLog, _ int) *entity.AuditEntry {
		return fromDBEntry(ctx, item)
	}), total, nil
}

func (s *Store) countLogs(ctx context.Context, where postgres.BoolExpression) (int64, error) {
	stmt := table.AuditLog.
		SELECT(postgres.COUNT(postgres.STAR).AS("count")).
		WHERE(where)

	var dest struct {
		Count int64
	}
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &dest); err != nil {
		return 0, err
	}
	return dest.Count, nil
}

func filterToWhereExp(f *entity.AuditFilter) postgres.BoolExpression {
	cond := postgres.Bool(true)
	if f == nil {
		return cond
	}

	if len(f.Actions) > 0 {
		cond = cond.AND(table.AuditLog.Action.IN(
			lo.Map(f.Actions, func(action entity.AuditAction, _ int) postgres.Expression {
				return postgres.String(string(action))
			})...,
		))
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
