package maint

import (
	"context"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/utils/xvalidation"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

const (
	mixStepDurationsMin = 5

	// maxNotifyTargets caps how many notify targets a maintenance may
	// have. Enforced here (domain invariant), not only at the HTTP
	// boundary, so non-HTTP callers can't exceed it.
	maxNotifyTargets = 100
)

func (s *Service) CreateDraft(ctx context.Context, cmd *entity.CreateMaintenanceCmd) (*entity.Maintenance, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.CreateDraft")
	defer span.End()

	if err := validateCreate(ctx, cmd); err != nil {
		return nil, err
	}

	var maint *entity.Maintenance
	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		maint, err = s.maintStore.CreateMaint(ctx, &entity.Maintenance{
			Title:         cmd.Title,
			Description:   cmd.Description,
			PlannedPeriod: cmd.PlannedPeriod,
			Scope:         cmd.Scope,
			Impact:        cmd.Impact,
			Status:        entity.MaintenanceStatusDraft,
		})
		if err != nil {
			xlog.Error(ctx, "create maint failed", xfield.Error(err))
			return err
		}

		maint.Resources = cmd.Resources
		maint.Normalize()
		if len(maint.Resources) > 0 {
			err = s.maintStore.AddResources(ctx, maint.ID, maint.Resources)
			if err != nil {
				xlog.Error(ctx, "attach resources to maint failed", xfield.Error(err))
				return err
			}
		}

		steps, err := s.maintStore.AddSteps(ctx, maint.ID, stepsFromCmd(cmd.Steps, entity.MaintenanceStepStatusPlanned))
		if err != nil {
			xlog.Error(ctx, "create maint steps failed", xfield.Error(err))
			return err
		}
		maint.Steps = steps

		notifyTargets, err := s.notifyTargets.Create(ctx, maint.ID, cmd.NotifyTargets)
		if err != nil {
			xlog.Error(ctx, "create notify targets failed", xfield.Error(err))
			return err
		}
		maint.NotifyTargets = notifyTargets

		return nil
	})
	if err != nil {
		xlog.Error(ctx, "create maint failed", xfield.Error(err))
		return nil, err
	}

	return maint, nil
}

func recalculatePlannedPeriod(plannedPeriodStart time.Time, steps []*entity.MaintenanceStep) entity.Period {
	totalDuration := lo.SumBy(steps, func(item *entity.MaintenanceStep) time.Duration {
		return time.Duration(item.DurationMinutes) * time.Minute
	})
	if totalDuration == 0 {
		return entity.NewOpenEndedPeriod(plannedPeriodStart)
	}

	return entity.NewPeriod(plannedPeriodStart, plannedPeriodStart.Add(totalDuration))
}

func validateCreate(ctx context.Context, cmd *entity.CreateMaintenanceCmd) error {
	return validation.ValidateStructWithContext(ctx, cmd,
		validation.Field(&cmd.Title, validation.Required),
		validation.Field(&cmd.Resources,
			validation.Required.When(cmd.Scope == entity.MaintenanceScopeResources),
			validation.Each(validation.By(xvalidation.UUIDNotNil)),
		),
		validation.Field(&cmd.Description, validation.Required),
		validation.Field(&cmd.PlannedPeriod, validation.Required,
			validation.WithContext(validatePlanedPeriod),
		),
		validation.Field(&cmd.Impact, validation.Required),
		validation.Field(&cmd.Scope, validation.Required),
		validation.Field(&cmd.Steps, validation.Required,
			validation.Each(validation.WithContext(validateStepInput))),
		validation.Field(&cmd.NotifyTargets, validation.Required,
			validation.Length(1, maxNotifyTargets),
			validation.Each(validation.WithContext(validateNotifyTargetsInput)),
		),
	)
}

func validatePlanedPeriod(_ context.Context, value any) error {
	period, err := xvalidation.Parse[entity.Period](value)
	if err != nil {
		return err
	}

	start := period.Start
	end := lo.FromPtr(period.End)

	if start.IsZero() || end.IsZero() {
		return apperr.ErrInvalidPeriodEmptyStartOrEnd
	}
	if start.After(end) || start.Equal(end) {
		return apperr.ErrInvalidPeriodStartOrEnd
	}

	return nil
}

func validateStepInput(ctx context.Context, value any) error {
	step, err := xvalidation.Parse[entity.MaintenanceStepInput](value)
	if err != nil {
		return err
	}

	return validation.ValidateStructWithContext(ctx, step,
		validation.Field(&step.Order, validation.Required, validation.Min(1)),
		validation.Field(&step.Description, validation.Required),
		validation.Field(&step.RollbackDescription, validation.Required),
		validation.Field(&step.DurationMinutes, validation.Required, validation.Min(mixStepDurationsMin)),
	)
}

func validateNotifyTargetsInput(ctx context.Context, value any) error {
	targets, err := xvalidation.Parse[entity.NotifyTargetInput](value)
	if err != nil {
		return err
	}

	return validation.ValidateStructWithContext(ctx, targets,
		validation.Field(&targets.ChannelID, validation.Required),
	)
}
