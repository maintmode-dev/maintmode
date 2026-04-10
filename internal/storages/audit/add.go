package audit

import (
	"context"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) AddLog(ctx context.Context, entry *entity.AuditEntry) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Audit.AddLog")
	defer span.End()

	stmt := table.AuditLog.
		INSERT(table.AuditLog.MutableColumns.
			Except(
				table.AuditLog.CreatedAt,
			),
		).
		MODEL(toDBEntry(entry))

	_, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return err
	}

	return nil
}
