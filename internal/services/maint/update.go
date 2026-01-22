package maint

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) Update(ctx context.Context, cmd *entity.UpdateMaintenanceCmd) error {
	ctx = xlog.WithOperation(ctx, "service.Maint.Update")

	return s.updateWithApply(ctx, cmd.MaintID, func(ctx context.Context, maint *entity.Maintenance) error {
		if maint.Status != entity.MaintenanceStatusDraft {
			return apperr.ForbiddenStatusTransition(maint.Status)
		}
		applyValuedFromUpdateCmd(maint, cmd)

		if len(maint.Resources) > 0 {
			if err := s.maintStore.DeleteResources(ctx, maint.ID); err != nil {
				return err
			}

			if err := s.maintStore.AddResources(ctx, maint.ID, maint.Resources); err != nil {
				return err
			}
		}

		return nil
	})
}

func applyValuedFromUpdateCmd(maint *entity.Maintenance, cmd *entity.UpdateMaintenanceCmd) {
	if cmd.Title != nil {
		maint.Title = lo.FromPtr(cmd.Title)
	}

	if cmd.Description != nil {
		maint.Description = lo.FromPtr(cmd.Description)
	}

	//Resources     []*Resource
	if cmd.PlannedPeriod != nil {
		maint.PlannedPeriod = lo.FromPtr(cmd.PlannedPeriod)
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

func (s *Service) updateWithApply(ctx context.Context, maintID uuid.UUID, apply func(ctx context.Context, maint *entity.Maintenance) error) error {
	return s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		maint, err := s.maintStore.GetForUpdate(ctx, maintID)
		if err != nil {
			return err
		}

		err = apply(ctx, maint)
		if err != nil {
			return err
		}

		err = s.maintStore.Update(ctx, maint)
		if err != nil {
			return err
		}

		return nil
	})
}
