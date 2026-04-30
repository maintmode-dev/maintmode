package maintenances

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) GetMaintSteps(ctx context.Context, maintID uuid.UUID) ([]*entity.MaintenanceStep, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Maintenances.GetMaintSteps")
	defer span.End()

	stmt := table.MaintenanceSteps.
		SELECT(table.MaintenanceSteps.AllColumns).
		WHERE(table.MaintenanceSteps.MaintenanceID.EQ(postgres.UUID(maintID))).
		ORDER_BY(table.MaintenanceSteps.StepOrder.ASC())

	return s.getSteps(ctx, stmt)
}

func (s *Store) GetMaintStepsForUpdate(ctx context.Context, maintID uuid.UUID) ([]*entity.MaintenanceStep, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Maintenances.GetMaintStepsForUpdate")
	defer span.End()

	stmt := table.MaintenanceSteps.
		SELECT(table.MaintenanceSteps.AllColumns).
		WHERE(table.MaintenanceSteps.MaintenanceID.EQ(postgres.UUID(maintID))).
		ORDER_BY(table.MaintenanceSteps.StepOrder.ASC()).
		FOR(postgres.UPDATE())

	return s.getSteps(ctx, stmt)
}

func (s *Store) getSteps(ctx context.Context, stmt postgres.Statement) ([]*entity.MaintenanceStep, error) {
	steps := make([]*model.MaintenanceSteps, 0)
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), &steps)
	if err != nil {
		return nil, err
	}

	return lo.Map(steps, func(item *model.MaintenanceSteps, _ int) *entity.MaintenanceStep {
		return fromDBMaintenanceStep(item)
	}), nil
}
