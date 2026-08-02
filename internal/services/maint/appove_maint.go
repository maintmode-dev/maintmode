package maint

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

func (s *Service) ApproveMaint(ctx context.Context, cmd *entity.ApproveMaintenanceCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.Approve")
	defer span.End()

	// The approve decision reads the actor's roles, so a missing actor must fail
	// here rather than degrade to the strict assignment check inside the tx.
	if cmd.Actor == nil {
		xlog.Error(ctx, "approve called without an actor")
		return apperr.ErrNilActor
	}

	// Approve runs in one SERIALIZABLE transaction: the conflict set is
	// recomputed *inside* the tx, after the maintenance row is locked, so a
	// concurrent approval that would change that set forces a serialization
	// abort and a retry instead of letting both windows snapshot a stale,
	// mutually-incomplete view. FOR UPDATE alone is not enough — it locks only
	// this row, not the conflicting maintenances or new inserts.
	_, current, err := s.updateWithApplySerializable(ctx, cmd.MaintID, func(ctx context.Context, maint *entity.Maintenance) error {
		if !entity.CanMaintTransition(maint.Status, entity.MaintenanceStatusPlanned) {
			return apperr.ForbiddenMaintStatusTransition(maint.Status, entity.MaintenanceStatusPlanned)
		}

		// Only the user assigned as approver may approve this maintenance. The
		// actor's role/permission is already enforced by RBAC; this guards the
		// assignment itself.
		//
		// Active admins are exempt: without an override a maintenance whose
		// approver left, was demoted or blocked would stay in draft forever, with
		// cancel (irreversible) as the only way out. The trade-off is deliberate —
		// approve no longer proves the assigned person decided — so the audit entry
		// records the acting admin, not the assignment.
		//
		// IsActiveAdmin, not IsAdmin: a blocked admin keeps the role but has lost
		// access. What actually rejects them today is the RequireActiveToken
		// middleware on this route, which introspects the subject against the
		// users table — AccessClaims carries no blocked flag, so BlockedAt is nil
		// on an actor built from a token and this call currently equals IsAdmin().
		// It is kept as the correct predicate for the question being asked, so a
		// future caller that does populate BlockedAt is denied by default rather
		// than inheriting the override.
		if maint.ApproverUserID != cmd.ActorUserID && !cmd.Actor.IsActiveAdmin() {
			return apperr.ErrApproverMismatch
		}

		if maint.Revision() != cmd.ObservedMaintRevision {
			return fmt.Errorf("%w: preview '%d', actual '%d'",
				apperr.ErrMaintChangedSincePreview,
				cmd.ObservedMaintRevision,
				maint.Revision(),
			)
		}

		// GetMaintForUpdate locks and loads the row only; the conflict query
		// needs the maintenance's resources, so load them inside the tx (part of
		// the same stable serializable view).
		if err := s.loadResources(ctx, maint); err != nil {
			xlog.Error(ctx, "failed to load maint resources", xfield.Error(err))
			return err
		}

		if err := s.checkConflicts(ctx, maint, cmd.ConflictSnapshot); err != nil {
			xlog.Error(ctx, "failed to check conflicts", xfield.Error(err))
			return err
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

	if err != nil {
		// Retries were exhausted under contention: hand the client a clean,
		// retryable domain error instead of leaking the raw serialization failure
		// (which would map to 500).
		if dbtx.ErrorIs(err, dbtx.ErrPGSerializationFailure) {
			xlog.Warn(ctx, "approve exhausted serialization retries", xfield.Error(err))
			return apperr.ErrConcurrentModification
		}
		return err
	}

	s.publishAudit(ctx, audit.MaintApproved{Actor: cmd.Actor, Maint: current})
	return nil
}

// loadResources fills maint.Resources from the join table. GetMaintForUpdate
// returns only the maintenance row, so callers that need the resource set (the
// conflict query) must load it explicitly.
func (s *Service) loadResources(ctx context.Context, maint *entity.Maintenance) error {
	resources, err := s.maintStore.GetMaintResources(ctx, []uuid.UUID{maint.ID})
	if err != nil {
		return err
	}

	maint.Resources = lo.Map(resources[maint.ID], func(r *entity.ResourceDetails, _ int) uuid.UUID {
		return r.ID
	})
	return nil
}

// checkConflicts recomputes the live conflict set for the (already locked)
// maintenance and rejects the approval when it no longer matches the set the
// approver previewed. Run inside the SERIALIZABLE approve transaction so the
// recomputed set is a stable, current view.
func (s *Service) checkConflicts(ctx context.Context, maint *entity.Maintenance, snapshot entity.ConflictsSnapshot) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.checkConflicts")
	defer span.End()

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
	actualFingerprint := entity.ConflictFingerprint(snapshot.Conflicts)
	if actualFingerprint != conflictsFingerprint {
		return fmt.Errorf("%w: preview '%s', actual '%s'",
			apperr.ErrConflictsChangedSincePreview,
			actualFingerprint,
			conflictsFingerprint,
		)
	}

	return nil
}
