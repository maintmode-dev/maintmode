package maint

import (
	"cmp"
	"context"
	"errors"

	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) CompleteMaint(ctx context.Context, cmd *entity.CompleteMaintenanceCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.Complete")
	defer span.End()

	err := s.updateWithApply(ctx, cmd.MaintID, func(ctx context.Context, maint *entity.Maintenance) error {
		// Already completed: idempotent no-op.
		if maint.Status == entity.MaintenanceStatusCompleted {
			return errSkipUpdate
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

		maint.ActualPeriod = cmp.Or(maint.ActualPeriod, &maint.PlannedPeriod)
		maint.ActualPeriod.End = lo.ToPtr(xtime.UTCNow())
		maint.Status = entity.MaintenanceStatusCompleted
		return nil
	})
	if errors.Is(err, errSkipUpdate) {
		return nil
	}

	return err
}

func allStepsTerminal(steps []*entity.MaintenanceStep) bool {
	for _, step := range steps {
		if !step.Status.IsTerminal() {
			return false
		}
	}
	return true
}
