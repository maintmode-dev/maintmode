package maint

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// StartStep transitions a maintenance step to "started".
//
//nolint:dupl // start/complete/cancel step intentionally share one shape; each sets a distinct status + audit action.
func (s *Service) StartStep(ctx context.Context, cmd *entity.StartMaintenanceStepCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.StartStep")
	defer span.End()

	maint, curr, err := s.updateStepWithApply(ctx, cmd.MaintID,
		func(_ context.Context, steps []*entity.MaintenanceStep) ([]*entity.MaintenanceStep, error) {
			step, err := canChangeStepStatusTo(steps, cmd.StepID, entity.MaintenanceStepStatusStarted)
			if err != nil {
				return nil, err
			}

			step.ActualStartedAt = lo.ToPtr(xtime.UTCNow())
			step.Status = entity.MaintenanceStepStatusStarted

			return []*entity.MaintenanceStep{step}, nil
		},
	)
	if err != nil {
		xlog.Error(ctx, "failed to start step", xfield.Error(err))
		return err
	}

	lo.ForEach(curr, func(item *entity.MaintenanceStep, _ int) {
		s.publishAudit(ctx, audit.MaintStepStarted{Actor: cmd.Actor, Maint: maint, Step: item})
	})

	return nil
}

// CompleteStep transitions a maintenance step to "completed".
//
//nolint:dupl // see StartStep — same intentional shape, distinct status/action.
func (s *Service) CompleteStep(ctx context.Context, cmd *entity.CompleteMaintenanceStepCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.CompleteStep")
	defer span.End()

	maint, curr, err := s.updateStepWithApply(ctx, cmd.MaintID,
		func(_ context.Context, steps []*entity.MaintenanceStep) ([]*entity.MaintenanceStep, error) {
			step, err := canChangeStepStatusTo(steps, cmd.StepID, entity.MaintenanceStepStatusCompleted)
			if err != nil {
				return nil, err
			}

			step.ActualCompletedAt = lo.ToPtr(xtime.UTCNow())
			step.Status = entity.MaintenanceStepStatusCompleted

			return []*entity.MaintenanceStep{step}, nil
		},
	)
	if err != nil {
		xlog.Error(ctx, "failed to complete step", xfield.Error(err))
		return err
	}

	lo.ForEach(curr, func(item *entity.MaintenanceStep, _ int) {
		s.publishAudit(ctx, audit.MaintStepCompleted{Actor: cmd.Actor, Maint: maint, Step: item})
	})

	return nil
}

// CancelStep transitions a maintenance step to "canceled".
//
//nolint:dupl // see StartStep — same intentional shape, distinct status/action.
func (s *Service) CancelStep(ctx context.Context, cmd *entity.CancelMaintenanceStepCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.CancelStep")
	defer span.End()

	maint, curr, err := s.updateStepWithApply(ctx, cmd.MaintID,
		func(_ context.Context, steps []*entity.MaintenanceStep) ([]*entity.MaintenanceStep, error) {
			step, err := canChangeStepStatusTo(steps, cmd.StepID, entity.MaintenanceStepStatusCanceled)
			if err != nil {
				return nil, err
			}

			step.ActualCompletedAt = lo.ToPtr(xtime.UTCNow())
			step.Status = entity.MaintenanceStepStatusCanceled

			return []*entity.MaintenanceStep{step}, nil
		},
	)
	if err != nil {
		xlog.Error(ctx, "failed to cancel step", xfield.Error(err))
		return err
	}

	lo.ForEach(curr, func(item *entity.MaintenanceStep, _ int) {
		s.publishAudit(ctx, audit.MaintStepCancelled{Actor: cmd.Actor, Maint: maint, Step: item})
	})

	return nil
}

func (s *Service) updateStepWithApply(
	ctx context.Context, maintID uuid.UUID,
	applyF func(ctx context.Context, steps []*entity.MaintenanceStep) ([]*entity.MaintenanceStep, error),
) (current *entity.Maintenance, currSteps []*entity.MaintenanceStep, err error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.updateStepWithApply")
	defer span.End()

	// currSteps is captured from the apply closure (the step set isn't part of
	// the maintenance snapshot updateWithApply returns); the maintenance itself
	// comes back as `current`.
	_, current, err = s.updateWithApply(ctx, maintID, func(ctx context.Context, maint *entity.Maintenance) error {
		if maint.Status != entity.MaintenanceStatusInProgress {
			err := fmt.Errorf("%w: maint is not in %s status",
				apperr.ErrForbiddenStepStatusTransition,
				entity.MaintenanceStatusInProgress,
			)
			xlog.Error(ctx, "maint is not in progress", xfield.Error(err))
			return err
		}

		steps, err := s.maintStore.GetMaintStepsForUpdate(ctx, maintID)
		if err != nil {
			xlog.Error(ctx, "failed to get maint steps for update", xfield.Error(err))
			return err
		}

		stepsForUpdate, err := applyF(ctx, steps)
		if err != nil {
			xlog.Error(ctx, "failed to apply step transition", xfield.Error(err))
			return err
		}

		currSteps = stepsForUpdate

		for _, step := range stepsForUpdate {
			if err := s.maintStore.UpdateStep(ctx, maintID, step); err != nil {
				xlog.Error(ctx, "failed to update step", xfield.Error(err))
				return err
			}
		}

		return nil
	})
	if err != nil {
		xlog.Error(ctx, "failed to update maint", xfield.Error(err))
		return nil, nil, err
	}

	// A step transition doesn't change maint status, so updateWithApply's own
	// lifecycle dispatch is a no-op here; notify is per-step.
	lo.ForEach(currSteps, func(item *entity.MaintenanceStep, _ int) {
		s.dispatchStepLifecycle(ctx, current, item)
	})
	return current, currSteps, nil
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

func (s *Service) dispatchStepLifecycle(ctx context.Context, maint *entity.Maintenance, step *entity.MaintenanceStep) {
	var eventType entity.NotifyEventKind
	switch {
	case maint.Status == entity.MaintenanceStatusInProgress && step.Status == entity.MaintenanceStepStatusStarted:
		eventType = entity.NotifyEventStepStarted
	case maint.Status == entity.MaintenanceStatusInProgress && step.Status == entity.MaintenanceStepStatusCompleted:
		eventType = entity.NotifyEventStepCompleted
	case maint.Status == entity.MaintenanceStatusInProgress && step.Status == entity.MaintenanceStepStatusCanceled:
		eventType = entity.NotifyEventStepCancelled
	default:
		return
	}

	s.notifier.NotifyStepLifecycle(ctx, eventType, maint, step)
}
