package maintenances

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func (s *Store) UpdateStep(ctx context.Context, maintID uuid.UUID, step *entity.MaintenanceStep) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Maintenances.UpdateStep")
	defer span.End()

	step.UpdatedAt = lo.ToPtr(xtime.UTCNow())

	stmt := table.MaintenanceSteps.
		UPDATE(table.MaintenanceSteps.MutableColumns).
		MODEL(toDBMaintenanceStep(maintID, step)).
		WHERE(
			table.MaintenanceSteps.ID.EQ(postgres.UUID(step.ID)).
				AND(table.MaintenanceSteps.MaintenanceID.EQ(postgres.UUID(maintID))),
		)

	_, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return err
	}

	return nil
}
