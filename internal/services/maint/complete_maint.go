package maint

import (
	"cmp"
	"context"
	"errors"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) CompleteMaint(ctx context.Context, cmd *entity.CompleteMaintenanceCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.Complete")
	defer span.End()

	_, current, err := s.updateWithApply(ctx, cmd.MaintID, func(ctx context.Context, maint *entity.Maintenance) error {
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

		// CloseActualPeriod rather than a direct end assignment: the fallback
		// planned period can start in the future, and writing an inverted actual
		// period would fail the update (see CloseActualPeriod).
		maint.ActualPeriod = cmp.Or(maint.ActualPeriod, &maint.PlannedPeriod)
		maint.CloseActualPeriod(xtime.UTCNow())
		maint.Status = entity.MaintenanceStatusCompleted
		return nil
	})
	if err != nil {
		if errors.Is(err, errSkipUpdate) {
			return nil
		}
		return err
	}

	s.publishAudit(ctx, audit.MaintCompleted{Actor: cmd.Actor, Maint: current})
	return nil
}

func allStepsTerminal(steps []*entity.MaintenanceStep) bool {
	for _, step := range steps {
		if !step.Status.IsTerminal() {
			return false
		}
	}
	return true
}
