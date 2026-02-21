package maintenances

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) DeleteResources(ctx context.Context, maintID uuid.UUID) error {
	ctx = xlog.WithOperation(ctx, "store.Maintenance.DeleteResources")

	stmt := table.MaintenanceResources.DELETE().
		WHERE(table.MaintenanceResources.MaintenanceID.EQ(postgres.UUID(maintID)))

	_, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return err
	}

	return nil
}
