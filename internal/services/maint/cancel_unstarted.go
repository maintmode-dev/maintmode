package maint

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
)

// cancelUnStartedReasonComment is the comment stored on maintenances canceled by
// the sweep. It carries no number so it stays correct regardless of the
// (configurable) grace window.
const cancelUnStartedReasonComment = "Automatically canceled: maintenance was not started within the grace period after its planned start"

// CancelUnStarted cancels maintenances that never started in time: those still in
// a not-started status (draft or planned) whose planned start is before cutoff.
// limit bounds how many are canceled in this call; the periodic job runs again
// soon, so any overflow is drained on the next tick. cutoff and limit come from
// the cron task payload (derived from config), keeping this method free of
// scheduling policy.
//
// The candidate list is an unlocked snapshot; each maintenance is then canceled
// under a FOR UPDATE re-read that asserts it is *still* not started (see
// cancelOneUnStarted). This closes the window where an operator approves and
// starts an overdue maintenance between the sweep query and the cancel: a
// maintenance that has since started, completed, or been canceled is skipped, so
// the job never cancels running work nor mislabels it "not_started". A failure on
// one maintenance is logged and does not abort the rest of the batch; the joined
// error is returned so the queue can retry.
func (s *Service) CancelUnStarted(ctx context.Context, cutoff time.Time, limit int64) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.CancelUnStarted")
	defer span.End()

	overdue, err := s.maintStore.ListOverdueNotStarted(ctx, cutoff, limit)
	if err != nil {
		xlog.Error(ctx, "failed to list overdue not-started maints", xfield.Error(err))
		return err
	}

	if len(overdue) == 0 {
		return nil
	}

	xlog.Info(ctx, "auto-canceling overdue not-started maints", xfield.Int("count", len(overdue)))

	var errs error
	for _, maint := range overdue {
		cmd := &entity.CancelMaintenanceCmd{
			MaintID:       maint.ID,
			Reason:        entity.MaintenanceCancelReasonNotStarted,
			ReasonComment: cancelUnStartedReasonComment,
			Actor:         entity.SystemUser,
		}
		cancelErr := s.cancelMaint(ctx, cmd, func(maint *entity.Maintenance) error {
			if maint.Status != entity.MaintenanceStatusDraft && maint.Status != entity.MaintenanceStatusPlanned {
				xlog.Warn(ctx, "auto-cancel skipped: maintenance already started or finished",
					xfield.String("maintID", maint.ID.String()),
					xfield.String("status", string(maint.Status)),
				)
				return errSkipUpdate
			}

			return nil
		})
		if cancelErr != nil {
			errs = errors.Join(errs, cancelErr)
		}
	}

	return errs
}

// cancelOneUnStarted cancels one maintenance only if it is still in a not-started
// status (draft or planned) when re-read under the row lock. Unlike the
// operator-facing CancelMaint (which may also cancel an in_progress maintenance),
// the sweep must never touch a maintenance that has already started — doing so
// would kill running work and mislabel it "not_started". A maintenance that
// changed status out from under the snapshot is left untouched (skipped via
// errSkipUpdate: no write, no lifecycle dispatch).
//
// It reuses updateWithApply, so the cancellation notification is emitted by the
// shared dispatchMaintLifecycle: planned->canceled notifies, while draft->canceled
// stays silent (a draft was never approved or announced to notify targets).
func (s *Service) cancelOneUnStarted(ctx context.Context, maintID uuid.UUID) error {
	_, current, err := s.updateWithApply(ctx, maintID, func(ctx context.Context, maint *entity.Maintenance) error {
		// Re-check under the lock: skip anything that is no longer not-started
		// (started, completed, or already canceled between the snapshot and now).
		if maint.Status != entity.MaintenanceStatusDraft && maint.Status != entity.MaintenanceStatusPlanned {
			xlog.Warn(ctx, "auto-cancel skipped: maintenance already started or finished",
				xfield.String("maintID", maintID.String()),
				xfield.String("status", string(maint.Status)),
			)
			return errSkipUpdate
		}

		maint.Status = entity.MaintenanceStatusCancelled
		maint.CancelReason = entity.MaintenanceCancelReasonNotStarted
		maint.CancelReasonComment = cancelUnStartedReasonComment
		return nil
	})
	if err != nil {
		if errors.Is(err, errSkipUpdate) {
			return nil
		}
		xlog.Error(ctx, "failed to cancel maint", xfield.Error(err))
		return err
	}

	// No human actor: the sweep is automated, so the audited actor is the
	// synthetic entity.SystemUser (RUK-182).
	s.publishAudit(ctx, audit.MaintCancelled{Actor: entity.SystemUser, Maint: current})
	return nil
}
