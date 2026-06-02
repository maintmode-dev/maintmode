package deferrednotifications

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// Replace swaps a maintenance's deferred-notification schedule for a new set.
// Must run inside the caller's tx (UpdateMaint).
//
// Edits are only allowed while a maintenance is still a draft, so no reminders
// are enqueued yet (goque_task_id is always NULL) — they appear on the next
// approve. cancelEnqueued is therefore a no-op here today; it is kept as
// defense-in-depth in case the draft-only edit invariant ever loosens. Because
// it no-ops in practice, its best-effort error handling has no tx-correctness
// impact on this draft path.
func (s *Service) Replace(ctx context.Context, maintID uuid.UUID, notifications []*entity.DeferredNotification) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.DeferredNotifications.Replace")
	defer span.End()

	if err := s.cancelEnqueued(ctx, maintID); err != nil {
		xlog.Error(ctx, "failed to cancel deferred notifications", xfield.Error(err))
		return err
	}

	if err := s.deferredNotificationsStore.DeleteByMaint(ctx, maintID); err != nil {
		xlog.Error(ctx, "failed to delete deferred notifications", xfield.Error(err))
		return err
	}

	_, err := s.deferredNotificationsStore.CreateMany(ctx, maintID, notifications)
	if err != nil {
		xlog.Error(ctx, "failed to recreate deferred notifications", xfield.Error(err))
		return err
	}

	return nil
}
