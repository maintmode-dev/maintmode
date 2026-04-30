package maint

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func (s *Service) StartStep(ctx context.Context, cmd *entity.StartMaintenanceStepCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.StartStep")
	defer span.End()

	return s.updateStepWithApply(ctx, cmd.MaintID, func(_ context.Context, steps []*entity.MaintenanceStep) ([]*entity.MaintenanceStep, error) {
		step, err := canChangeStepStatusTo(steps, cmd.StepID, entity.MaintenanceStepStatusStarted)
		if err != nil {
			return nil, err
		}

		step.ActualStartedAt = lo.ToPtr(xtime.UTCNow())
		step.Status = entity.MaintenanceStepStatusStarted

		return []*entity.MaintenanceStep{step}, nil
	})
}

func (s *Service) CompleteStep(ctx context.Context, cmd *entity.CompleteMaintenanceStepCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.CompleteStep")
	defer span.End()

	return s.updateStepWithApply(ctx, cmd.MaintID, func(_ context.Context, steps []*entity.MaintenanceStep) ([]*entity.MaintenanceStep, error) {
		step, err := canChangeStepStatusTo(steps, cmd.StepID, entity.MaintenanceStepStatusCompleted)
		if err != nil {
			return nil, err
		}

		step.ActualCompletedAt = lo.ToPtr(xtime.UTCNow())
		step.Status = entity.MaintenanceStepStatusCompleted

		return []*entity.MaintenanceStep{step}, nil
	})
}

func (s *Service) CancelStep(ctx context.Context, cmd *entity.CancelMaintenanceStepCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.CancelStep")
	defer span.End()

	return s.updateStepWithApply(ctx, cmd.MaintID, func(_ context.Context, steps []*entity.MaintenanceStep) ([]*entity.MaintenanceStep, error) {
		step, err := canChangeStepStatusTo(steps, cmd.StepID, entity.MaintenanceStepStatusCanceled)
		if err != nil {
			return nil, err
		}

		step.ActualCompletedAt = lo.ToPtr(xtime.UTCNow())
		step.Status = entity.MaintenanceStepStatusCanceled

		return []*entity.MaintenanceStep{step}, nil
	})
}

func (s *Service) updateStepWithApply(
	ctx context.Context, maintID uuid.UUID,
	applyF func(ctx context.Context, steps []*entity.MaintenanceStep) ([]*entity.MaintenanceStep, error),
) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.updateStepWithApply")
	defer span.End()

	return s.updateWithApply(ctx, maintID, func(ctx context.Context, maint *entity.Maintenance) error {
		if maint.Status != entity.MaintenanceStatusInProgress {
			return fmt.Errorf("%w: maint is not in %s status", apperr.ErrForbiddenStepStatusTransition, entity.MaintenanceStatusInProgress)
		}

		steps, err := s.maintStore.GetMaintStepsForUpdate(ctx, maintID)
		if err != nil {
			return err
		}

		stepsForUpdate, err := applyF(ctx, steps)
		if err != nil {
			return err
		}

		for _, step := range stepsForUpdate {
			if err := s.maintStore.UpdateStep(ctx, maintID, step); err != nil {
				return err
			}
		}

		return nil
	})
}

func canChangeStepStatusTo(steps []*entity.MaintenanceStep, stepID uuid.UUID, newStatus entity.MaintenanceStepStatus) (*entity.MaintenanceStep, error) {
	sorted := slices.SortedFunc(slices.Values(steps), func(s1 *entity.MaintenanceStep, s2 *entity.MaintenanceStep) int {
		return cmp.Compare(s1.Order, s2.Order)
	})

	step, stepIdx, ok := lo.FindIndexOf(sorted, func(item *entity.MaintenanceStep) bool {
		return item.ID == stepID
	})
	if !ok || stepIdx < 0 {
		return nil, apperr.ErrStepNotFound
	}

	// if step is NOT first -> check the status of the previous step.
	if stepIdx > 0 {
		// if the previous step status is not terminal -> current step can't be changed
		prevStep := sorted[stepIdx-1]
		if !prevStep.Status.IsTerminal() {
			return nil, apperr.ErrStepOrderViolation
		}
	}

	if !entity.CanMainStepTransition(step.Status, newStatus) {
		return nil, apperr.ForbiddenStepStatusTransition(step.Status, newStatus)
	}

	return step, nil
}
