package useridentities

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// ListProvidersByUserID returns the providers linked to userID, ordered by the
// identity creation time. There is at most one identity per (user, provider),
// so the result is already distinct.
func (s *Store) ListProvidersByUserID(ctx context.Context, userID uuid.UUID) ([]entity.OAuthProvider, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserIdentities.ListProvidersByUserID")
	defer span.End()

	stmt := table.UserIdentities.
		SELECT(postgres.DISTINCT(table.UserIdentities.Provider)).
		WHERE(table.UserIdentities.UserID.EQ(postgres.UUID(userID))).
		ORDER_BY(table.UserIdentities.CreatedAt.ASC(), table.UserIdentities.ID.ASC())

	var rows []*model.UserIdentities
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &rows); err != nil {
		return nil, err
	}

	return lo.Map(rows, func(item *model.UserIdentities, _ int) entity.OAuthProvider {
		return entity.OAuthProvider(item.Provider)
	}), nil
}

func (s *Store) CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserIdentities.CountByUserID")
	defer span.End()

	stmt := table.UserIdentities.
		SELECT(postgres.COUNT(postgres.STAR).AS("count")).
		WHERE(table.UserIdentities.UserID.EQ(postgres.UUID(userID)))

	var dest struct {
		Count int64
	}
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &dest); err != nil {
		return 0, err
	}

	return dest.Count, nil
}
