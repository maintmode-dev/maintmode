package notifytargets

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) Delete(ctx context.Context, maintID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.MaintenanceNotifyTargets.Delete")
	defer span.End()

	delStmt := table.MaintenanceNotifyTargets.DELETE().
		WHERE(table.MaintenanceNotifyTargets.MaintenanceID.EQ(postgres.UUID(maintID)))

	if _, err := delStmt.ExecContext(ctx, s.db.Executor(ctx)); err != nil {
		return err
	}

	return nil
}
