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

	// created_at is written explicitly from the event's occurred-at time:
	// ordering must reflect when the event happened, not when the
	// drain processor inserted the row. id stays a DB-generated uuidv7 PK; GetLogs
	// uses it as the created_at tie-breaker for stable ordering.
	//
	// ON CONFLICT (event_id) DO NOTHING makes the write idempotent: if the
	// processor already inserted this event but its ack was lost and the task
	// is redelivered, the retry collapses to a no-op instead of duplicating the
	// audit record. Legacy rows have a NULL event_id and never conflict.
	stmt := table.AuditLog.
		INSERT(table.AuditLog.MutableColumns).
		MODEL(toDBEntry(ctx, entry)).
		ON_CONFLICT(table.AuditLog.EventID).
		DO_NOTHING()

	_, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return err
	}

	return nil
}
