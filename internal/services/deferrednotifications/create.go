package deferrednotifications

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// Create resolves and persists a maintenance's deferred notifications. Must run
// inside the caller's tx (CreateDraft). No reminders are enqueued here —
// enqueueing happens on approve.
func (s *Service) Create(ctx context.Context, maintID uuid.UUID, notifications []*entity.DeferredNotification) ([]*entity.DeferredNotification, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.DeferredNotifications.Create")
	defer span.End()

	created, err := s.deferredNotificationsStore.CreateMany(ctx, maintID, notifications)
	if err != nil {
		xlog.Error(ctx, "failed to create deferred notifications", xfield.Error(err))
		return nil, err
	}

	return created, nil
}
