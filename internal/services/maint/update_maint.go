package maint

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) UpdateMaint(ctx context.Context, cmd *entity.UpdateMaintenanceCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.Update")
	defer span.End()

	if err := validateUpdate(ctx, cmd); err != nil {
		return err
	}

	return s.updateWithApply(ctx, cmd.MaintID, func(ctx context.Context, maint *entity.Maintenance) error {
		if maint.Status != entity.MaintenanceStatusDraft {
			return apperr.ForbiddenMaintStatusTransition(maint.Status, entity.MaintenanceStatusDraft)
		}
		applyValuesFromUpdateCmd(maint, cmd)

		if len(cmd.Steps) > 0 {
			if err := s.replaceSteps(ctx, maint); err != nil {
				xlog.Error(ctx, "replace steps failed", xfield.Error(err))
				return err
			}
		}

		if len(cmd.Resources) > 0 || lo.FromPtr(cmd.Scope) == entity.MaintenanceScopeGlobal {
			if err := s.replaceResources(ctx, maint); err != nil {
				xlog.Error(ctx, "replace resources failed", xfield.Error(err))
				return err
			}
		}

		return nil
	})
}

func applyValuesFromUpdateCmd(maint *entity.Maintenance, cmd *entity.UpdateMaintenanceCmd) {
	if cmd.Title != nil {
		maint.Title = lo.FromPtr(cmd.Title)
	}

	if cmd.Description != nil {
		maint.Description = lo.FromPtr(cmd.Description)
	}

	if len(cmd.Steps) != 0 {
		maint.Steps = lo.Map(cmd.Steps, func(item *entity.MaintenanceStepInput, _ int) *entity.MaintenanceStep {
			return &entity.MaintenanceStep{
				Order:               item.Order,
				Description:         item.Description,
				RollbackDescription: item.RollbackDescription,
				DurationMinutes:     item.DurationMinutes,
				Status:              entity.MaintenanceStepStatusPlanned,
			}
		})
	}

	if len(cmd.Steps) > 0 {
		maint.PlannedPeriod = recalculatePlannedPeriod(maint.PlannedPeriod.Start, maint.Steps)
	}

	if cmd.PlannedStart != nil {
		currentDuration := maint.PlannedPeriod.Duration()

		newStart := lo.FromPtr(cmd.PlannedStart)
		newEnd := newStart.Add(currentDuration)

		maint.PlannedPeriod = entity.NewPeriod(newStart, newEnd)
	}

	if cmd.Scope != nil {
		maint.Scope = lo.FromPtr(cmd.Scope)
	}

	if cmd.Impact != nil {
		maint.Impact = lo.FromPtr(cmd.Impact)
	}

	if len(cmd.Resources) > 0 {
		maint.Resources = cmd.Resources
	}
}

func (s *Service) replaceResources(ctx context.Context, maint *entity.Maintenance) error {
	if err := s.maintStore.DeleteResources(ctx, maint.ID); err != nil {
		return err
	}
	if maint.Scope == entity.MaintenanceScopeGlobal {
		return nil
	}

	return s.maintStore.AddResources(ctx, maint.ID, maint.Resources)
}

func (s *Service) replaceSteps(ctx context.Context, maint *entity.Maintenance) error {
	if err := s.maintStore.DeleteSteps(ctx, maint.ID); err != nil {
		return err
	}

	_, err := s.maintStore.AddSteps(ctx, maint.ID, maint.Steps)
	return err
}

func validateUpdate(ctx context.Context, cmd *entity.UpdateMaintenanceCmd) error {
	return validation.ValidateStructWithContext(ctx, cmd,
		validation.Field(&cmd.Title, validation.NilOrNotEmpty),
		validation.Field(&cmd.Description, validation.NilOrNotEmpty),
		validation.Field(&cmd.PlannedStart, validation.NilOrNotEmpty),
		validation.Field(&cmd.Scope, validation.NilOrNotEmpty),
		validation.Field(&cmd.Impact, validation.NilOrNotEmpty),

		// validate only if changed
		validation.Field(&cmd.Resources, validation.Each(validation.WithContext(validateResource))),
		validation.Field(&cmd.Steps, validation.Each(validation.WithContext(validateStepInput))),
	)
}

func (s *Service) updateWithApply(ctx context.Context, maintID uuid.UUID, apply func(ctx context.Context, maint *entity.Maintenance) error) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.updateWithApply")
	defer span.End()

	return s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		maint, err := s.maintStore.GetMaintForUpdate(ctx, maintID)
		if err != nil {
			return err
		}

		err = apply(ctx, maint)
		if err != nil {
			return err
		}

		err = s.maintStore.UpdateMaint(ctx, maint)
		if err != nil {
			return err
		}

		return nil
	})
}
