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

// ListProvidersByUserID returns the providers linked to userID, ordered by
// provider ASC. There is at most one identity per (user, provider), so the
// result is already distinct.
func (s *Store) ListProvidersByUserID(ctx context.Context, userID uuid.UUID) ([]entity.OAuthProvider, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserIdentities.ListProvidersByUserID")
	defer span.End()

	stmt := table.UserIdentities.
		SELECT(table.UserIdentities.Provider).
		DISTINCT(table.UserIdentities.Provider).
		WHERE(table.UserIdentities.UserID.EQ(postgres.UUID(userID))).
		ORDER_BY(table.UserIdentities.Provider.ASC())

	var rows []*model.UserIdentities
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &rows); err != nil {
		return nil, err
	}

	return lo.Map(rows, func(item *model.UserIdentities, _ int) entity.OAuthProvider {
		return entity.OAuthProvider(item.Provider)
	}), nil
}

// ListProvidersByUserIDs returns the providers linked to each of userIDs, keyed
// by user ID, with providers ordered by provider ASC within each user so the
// first element is a deterministic primary (as in ListProvidersByUserID). Users
// with no identities are absent from the map. A single query avoids the N+1 that
// calling ListProvidersByUserID per row would incur on the admin user list.
func (s *Store) ListProvidersByUserIDs(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID][]entity.OAuthProvider, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserIdentities.ListProvidersByUserIDs")
	defer span.End()

	if len(userIDs) == 0 {
		return map[uuid.UUID][]entity.OAuthProvider{}, nil
	}

	idExprs := lo.Map(userIDs, func(id uuid.UUID, _ int) postgres.Expression {
		return postgres.UUID(id)
	})

	// No DISTINCT needed: the user_identities_user_provider_uidx unique index
	// guarantees at most one row per (user_id, provider).
	stmt := table.UserIdentities.
		SELECT(table.UserIdentities.UserID, table.UserIdentities.Provider).
		WHERE(table.UserIdentities.UserID.IN(idExprs...)).
		ORDER_BY(table.UserIdentities.UserID.ASC(), table.UserIdentities.Provider.ASC())

	var rows []*model.UserIdentities
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &rows); err != nil {
		return nil, err
	}

	result := make(map[uuid.UUID][]entity.OAuthProvider, len(userIDs))
	for _, row := range rows {
		result[row.UserID] = append(result[row.UserID], entity.OAuthProvider(row.Provider))
	}

	return result, nil
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
