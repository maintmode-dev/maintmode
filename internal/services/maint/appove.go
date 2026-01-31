package maint

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) Approve(ctx context.Context, cmd *entity.ApproveMaintenanceCmd) error {
	ctx = xlog.WithOperation(ctx, "service.Maint.Approve")

	err := s.checkConflicts(ctx, cmd)
	if err != nil {
		return fmt.Errorf("checkConflicts: %w", err)
	}

	return s.updateWithApply(ctx, cmd.MaintID, func(_ context.Context, maint *entity.Maintenance) error {
		if !entity.CanTransition(maint.Status, entity.MaintenanceStatusPlanned) {
			return apperr.ForbiddenStatusTransition(maint.Status)
		}

		if maint.Revision() != cmd.ObservedMaintRevision {
			return fmt.Errorf("%w: preview '%d', actual '%d'",
				apperr.ErrMaintChangedSincePreview,
				cmd.ObservedMaintRevision,
				maint.Revision(),
			)
		}

		maint.Status = entity.MaintenanceStatusPlanned
		return nil
	})
}

func (s *Service) checkConflicts(ctx context.Context, cmd *entity.ApproveMaintenanceCmd) error {
	ctx = xlog.WithOperation(ctx, "service.Maint.CheckConflicts")

	maint, err := s.Get(ctx, cmd.MaintID)
	if err != nil {
		return err
	}

	conflicts, err := s.conflictsSrv.GetConflicts(ctx, &entity.ConflictQueryCmd{
		MaintID:       maint.ID,
		PlannedPeriod: maint.PlannedPeriod,
		Scope:         maint.Scope,
		ResourceIDs: lo.Map(maint.Resources, func(item *entity.Resource, _ int) uuid.UUID {
			return item.ID
		}),
	})
	if err != nil {
		return err
	}

	conflictsFingerprint := entity.ConflictFingerprint(conflicts)
	actualFingerprint := entity.ConflictFingerprint(cmd.ConflictSnapshot.Conflicts)
	if actualFingerprint != conflictsFingerprint {
		return fmt.Errorf("%w: preview '%s', actual '%s'",
			apperr.ErrConflictsChangedSincePreview,
			actualFingerprint,
			conflictsFingerprint,
		)
	}

	return nil
}
