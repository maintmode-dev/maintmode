package maint

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) ApproveMaint(ctx context.Context, cmd *entity.ApproveMaintenanceCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.Approve")
	defer span.End()

	err := s.checkConflicts(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "failed to check conflicts", xfield.Error(err))
		return fmt.Errorf("checkConflicts: %w", err)
	}

	return s.updateWithApply(ctx, cmd.MaintID, func(ctx context.Context, maint *entity.Maintenance) error {
		if !entity.CanMaintTransition(maint.Status, entity.MaintenanceStatusPlanned) {
			return apperr.ForbiddenMaintStatusTransition(maint.Status, entity.MaintenanceStatusPlanned)
		}

		// Only the user assigned as approver may approve this maintenance. The
		// actor's role/permission is already enforced by RBAC; this guards the
		// assignment itself.
		if maint.ApproverUserID != cmd.ActorUserID {
			return apperr.ErrApproverMismatch
		}

		if maint.Revision() != cmd.ObservedMaintRevision {
			return fmt.Errorf("%w: preview '%d', actual '%d'",
				apperr.ErrMaintChangedSincePreview,
				cmd.ObservedMaintRevision,
				maint.Revision(),
			)
		}

		if err := s.conflictsSrv.SaveSnapshot(ctx, &entity.SaveConflictsSnapshotCmd{
			MaintID:          cmd.MaintID,
			ConflictSnapshot: cmd.ConflictSnapshot,
		}); err != nil {
			xlog.Error(ctx, "failed to bulk insert conflict snapshots", xfield.Error(err))
			return fmt.Errorf("bulk insert conflict snapshots: %w", err)
		}

		maint.Status = entity.MaintenanceStatusPlanned

		// Schedule the deferred reminders in the same tx as the transition to
		// "planned": the scheduler joins this tx via the outbox, so the queued
		// tasks and the status change commit atomically — no crash window where
		// a scheduled maintenance has un-enqueued reminders.
		if err := s.deferred.Schedule(ctx, cmd.MaintID); err != nil {
			xlog.Error(ctx, "failed to enqueue deferred reminders", xfield.Error(err))
			return fmt.Errorf("enqueue deferred reminders: %w", err)
		}

		return nil
	})
}

func (s *Service) checkConflicts(ctx context.Context, cmd *entity.ApproveMaintenanceCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.checkConflicts")
	defer span.End()

	maint, err := s.GetMaint(ctx, cmd.MaintID)
	if err != nil {
		return err
	}

	conflicts, err := s.conflictsSrv.GetConflicts(ctx, &entity.ConflictQueryCmd{
		MaintID:       maint.ID,
		PlannedPeriod: maint.PlannedPeriod,
		Scope:         maint.Scope,
		ResourceIDs:   maint.Resources,
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
