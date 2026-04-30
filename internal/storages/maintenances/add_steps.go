package maintenances

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) AddSteps(ctx context.Context, maintID uuid.UUID, steps []*entity.MaintenanceStep) ([]*entity.MaintenanceStep, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Maintenances.AddSteps")
	defer span.End()

	if len(steps) == 0 {
		return nil, nil
	}

	dbSteps := lo.Map(steps, func(item *entity.MaintenanceStep, _ int) *model.MaintenanceSteps {
		return toDBMaintenanceStep(maintID, item)
	})

	stmt := table.MaintenanceSteps.
		INSERT(table.MaintenanceSteps.MutableColumns.
			Except(table.MaintenanceSteps.CreatedAt),
		).
		MODELS(dbSteps).
		RETURNING(table.MaintenanceSteps.AllColumns)

	result := make([]*model.MaintenanceSteps, 0, len(steps))
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), &result)
	if err != nil {
		return nil, err
	}

	return lo.Map(result, func(item *model.MaintenanceSteps, _ int) *entity.MaintenanceStep {
		return fromDBMaintenanceStep(item)
	}), nil
}
