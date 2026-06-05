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
	// maxSteps and minSteps caps how many notify steps a maintenance may have.
	maxSteps = 100
	minSteps = 1
	// minStepDurationsMinutes is minimum step duration
	minStepDurationsMinutes = 5

	// minNotifyTargets and maxNotifyTargets caps how many notify targets a maintenance may have.
	minNotifyTargets = 1
	maxNotifyTargets = 10

	// maxDeferredNotifications and minDeferredNotifications caps how many deferred reminders a maintenance may carry
	minDeferredNotifications = 0
	maxDeferredNotifications = 10
)

func (s *Service) CreateDraft(ctx context.Context, cmd *entity.CreateMaintenanceCmd) (*entity.Maintenance, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.CreateDraft")
	defer span.End()

	if err := validateCreate(ctx, cmd); err != nil {
		return nil, err
	}

	if err := s.validateApprover(ctx, cmd.ApproverUserID); err != nil {
		return nil, err
	}

	var maint *entity.Maintenance
	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		maint, err = s.maintStore.CreateMaint(ctx, &entity.Maintenance{
			Title:           cmd.Title,
			Description:     cmd.Description,
			PlannedPeriod:   cmd.PlannedPeriod,
			Scope:           cmd.Scope,
			Impact:          cmd.Impact,
			Status:          entity.MaintenanceStatusDraft,
			ApproverUserID:  cmd.ApproverUserID,
			CreatedByUserID: cmd.CreatedByUserID,
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

		deferred, err := s.deferred.Create(ctx, maint.ID, deferredNotificationsFromCmd(maint.ID, cmd.DeferredNotifications))
		if err != nil {
			xlog.Error(ctx, "create deferred notifications failed", xfield.Error(err))
			return err
		}
		maint.DeferredNotifications = deferred

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
		validation.Field(&cmd.CreatedByUserID, validation.Required),
		validation.Field(&cmd.ApproverUserID, validation.Required),
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
			validation.Length(minSteps, maxSteps),
			validation.Each(validation.WithContext(validateStepInput))),
		validation.Field(&cmd.NotifyTargets, validation.Required,
			validation.Length(minNotifyTargets, maxNotifyTargets),
			validation.Each(validation.WithContext(validateNotifyTargetsInput)),
		),
		// Deferred notifications are optional; validate each when present.
		validation.Field(&cmd.DeferredNotifications,
			validation.Length(minDeferredNotifications, maxDeferredNotifications),
			validation.Each(validation.WithContext(validateDeferredNotificationInput)),
		),
	)
}

func validateDeferredNotificationInput(ctx context.Context, value any) error {
	notification, err := xvalidation.Parse[entity.DeferredNotificationInput](value)
	if err != nil {
		return err
	}

	return validation.ValidateStructWithContext(ctx, notification,
		validation.Field(&notification.FireAt, validation.Required),
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
		validation.Field(&step.DurationMinutes, validation.Required, validation.Min(minStepDurationsMinutes)),
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
