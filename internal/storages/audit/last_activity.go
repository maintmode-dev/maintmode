package audit

import (
	"context"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

type lastActivityAt struct {
	LastActivityAt *time.Time `alias:"audit.last_activity_at"`
}

// LastActivityAt returns MAX(created_at) over the maintenance audit actions
// (entity.AuditCategoryAction(AuditCategoryMaintenance)) — real product work,
// not auth or admin bookkeeping. It feeds the license heartbeat's
// last_activity_at; nil means the instance has seen no such activity yet and
// the heartbeat reports null. Runs once per heartbeat tick; served by the
// composite (action, created_at DESC) index so even the no-maintenance case
// (zero matching rows) stays an index probe, not a table walk.
func (s *Store) LastActivityAt(ctx context.Context) (*time.Time, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Audit.LastActivityAt")
	defer span.End()

	stmt := table.AuditLog.
		SELECT(postgres.MAX(table.AuditLog.CreatedAt).AS("audit.last_activity_at")).
		WHERE(table.AuditLog.Action.IN(lo.Map(
			entity.AuditCategoryAction(entity.AuditCategoryMaintenance),
			func(action entity.AuditAction, _ int) postgres.Expression {
				return postgres.String(string(action))
			},
		)...))

	dest := new(lastActivityAt)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), dest); err != nil {
		return nil, err
	}
	return dest.LastActivityAt, nil
}
