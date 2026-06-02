package deferrednotifications

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// CreateMany persists a maintenance's deferred-notification schedule. Runs in
// the caller's tx (CreateDraft). goque_task_id stays NULL until approve.
//
// Empty input is a no-op, not an error: callers rely on this to skip the write
// when a maintenance carries no reminders.
func (s *Store) CreateMany(ctx context.Context, maintID uuid.UUID, notifications []*entity.DeferredNotification) ([]*entity.DeferredNotification, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.DeferredNotifications.CreateMany")
	defer span.End()

	if len(notifications) == 0 {
		return nil, nil
	}

	dbNotifications := lo.Map(notifications, func(n *entity.DeferredNotification, _ int) *model.MaintenanceDeferredNotifications {
		return toDB(maintID, n)
	})

	stmt := table.MaintenanceDeferredNotifications.
		INSERT(table.MaintenanceDeferredNotifications.MutableColumns.
			Except(table.MaintenanceDeferredNotifications.CreatedAt),
		).
		MODELS(dbNotifications).
		RETURNING(table.MaintenanceDeferredNotifications.AllColumns)

	inserted := make([]*model.MaintenanceDeferredNotifications, 0, len(notifications))
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &inserted); err != nil {
		return nil, err
	}

	return lo.Map(inserted, func(m *model.MaintenanceDeferredNotifications, _ int) *entity.DeferredNotification {
		return fromDB(m)
	}), nil
}
