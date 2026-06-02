package deferrednotifications

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
)

// ListByMaint returns a maintenance's deferred-notification schedule (for read
// paths such as GetMaint).
func (s *Service) ListByMaint(ctx context.Context, maintID uuid.UUID) ([]*entity.DeferredNotification, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.DeferredNotifications.ListByMaint")
	defer span.End()

	return s.deferredNotificationsStore.ListByMaint(ctx, maintID)
}
