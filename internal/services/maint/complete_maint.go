package maint

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) CompleteMaint(ctx context.Context, cmd *entity.CompleteMaintenanceCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.Complete")
	defer span.End()

	return s.updateWithApply(ctx, cmd.MaintID, func(ctx context.Context, maint *entity.Maintenance) error {
		if maint.Status == entity.MaintenanceStatusCompleted {
			return nil
		}

		if !entity.CanMaintTransition(maint.Status, entity.MaintenanceStatusCompleted) {
			return apperr.ForbiddenMaintStatusTransition(maint.Status, entity.MaintenanceStatusCompleted)
		}

		steps, err := s.maintStore.GetMaintSteps(ctx, maint.ID)
		if err != nil {
			return err
		}

		if !allStepsTerminal(steps) {
			return apperr.ErrMaintenanceHasUnfinishedSteps
		}

		maint.ActualPeriod.End = lo.ToPtr(xtime.UTCNow())
		maint.Status = entity.MaintenanceStatusCompleted
		return nil
	})
}

func allStepsTerminal(steps []*entity.MaintenanceStep) bool {
	for _, step := range steps {
		if !step.Status.IsTerminal() {
			return false
		}
	}
	return true
}
