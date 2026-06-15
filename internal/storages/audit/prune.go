package audit

import (
	"context"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// PruneOlderThan deletes up to limit audit_log rows whose created_at is strictly
// before cutoff and returns how many it removed. It is the single batch behind
// the retention sweep: the caller loops until a batch deletes fewer than limit,
// so one call must never delete more than limit rows.
//
// Postgres has no DELETE ... LIMIT, so the batch is bounded by an id-subquery:
// pick the oldest `limit` expired ids (ORDER BY created_at via the existing
// idx_audit_log_created_at), then delete exactly those. Bounding each batch keeps
// the per-statement lock footprint small on a table that may have grown large
// before retention was switched on.
func (s *Store) PruneOlderThan(ctx context.Context, cutoff time.Time, limit int64) (int64, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Audit.PruneOlderThan")
	defer span.End()

	// ORDER BY created_at ASC is served by idx_audit_log_created_at (created_at
	// DESC) via an index scan backward — Postgres reads a DESC btree in either
	// direction. The order and that index are coupled: dropping the index, or
	// flipping this to DESC, would force a seq scan + sort on a large table.
	expired := table.AuditLog.
		SELECT(table.AuditLog.ID).
		WHERE(table.AuditLog.CreatedAt.LT(postgres.TimestampzT(cutoff))).
		ORDER_BY(table.AuditLog.CreatedAt.ASC()).
		LIMIT(limit)

	stmt := table.AuditLog.
		DELETE().
		WHERE(table.AuditLog.ID.IN(expired))

	res, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}
