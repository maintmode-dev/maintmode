package maintenances

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) DeleteSteps(ctx context.Context, maintID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Maintenances.DeleteSteps")
	defer span.End()

	stmt := table.MaintenanceSteps.DELETE().
		WHERE(table.MaintenanceSteps.MaintenanceID.EQ(postgres.UUID(maintID)))

	_, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return err
	}
	return nil
}
