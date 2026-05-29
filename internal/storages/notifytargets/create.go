package notifytargets

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) CreateMany(ctx context.Context, maintID uuid.UUID, targets []*entity.NotifyTarget) ([]*entity.NotifyTarget, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.MaintenanceNotifyTargets.CreateMany")
	defer span.End()

	// Empty input is a no-op, not an error: callers (service Create /
	// Replace) rely on this to skip the INSERT when there's nothing
	// to persist.
	if len(targets) == 0 {
		return nil, nil
	}

	notifyTargets := lo.Map(targets, func(sub *entity.NotifyTarget, _ int) *model.MaintenanceNotifyTargets {
		return toDB(maintID, sub)
	})

	stmt := table.MaintenanceNotifyTargets.
		INSERT(
			table.MaintenanceNotifyTargets.MutableColumns.
				Except(
					table.MaintenanceNotifyTargets.CreatedAt,
				),
		).
		MODELS(notifyTargets).
		RETURNING(table.MaintenanceNotifyTargets.AllColumns)

	result := make([]*model.MaintenanceNotifyTargets, 0, len(targets))
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), &result)
	if err != nil {
		if dbtx.ErrorIs(err, dbtx.ErrPGUniqueViolation) {
			return nil, apperr.ErrNotifyTargetsAlreadyExists
		}
		return nil, err
	}

	return lo.Map(result, func(item *model.MaintenanceNotifyTargets, _ int) *entity.NotifyTarget {
		return fromDB(item)
	}), nil
}
