package conflictsnapshots

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

type conflictSnapshots struct {
	*model.MaintenanceConflictSnapshot
	*model.Maintenances
}

func (s *Store) GetSnapshots(ctx context.Context, maintID uuid.UUID) ([]*entity.ConflictWithResources, error) {
	ctx = xlog.WithOperation(ctx, "store.ConflictSnapshots.GetSnapshots")

	stmt := table.MaintenanceConflictSnapshot.
		INNER_JOIN(table.Maintenances, table.Maintenances.ID.EQ(table.MaintenanceConflictSnapshot.ConflictedMaintenanceID)).
		SELECT(
			table.MaintenanceConflictSnapshot.AllColumns,
			table.Maintenances.Scope,
			table.Maintenances.Title,
		).
		WHERE(table.MaintenanceConflictSnapshot.MaintenanceID.EQ(postgres.UUID(maintID)))

	snaphots := make([]*conflictSnapshots, 0)
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), &snaphots)
	if err != nil {
		return nil, err
	}

	return fromDBSnaphots(snaphots), nil
}
