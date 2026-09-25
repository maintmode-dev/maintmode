package useridentities

import (
	"context"
	"slices"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// identityMethodRow is one identity with the registry row behind it, scanned
// from a plain join of the two jet models.
type identityMethodRow struct {
	model.UserIdentities
	model.IntegrationSettings
}

// ListMethodsByUserID returns the sign-in providers linked to userID, ordered
// by name ASC. Break-glass is not among them; see listMethods.
func (s *Store) ListMethodsByUserID(ctx context.Context, userID uuid.UUID) ([]entity.AuthMethod, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserIdentities.ListMethodsByUserID")
	defer span.End()

	byUser, err := s.listMethods(ctx, table.UserIdentities.UserID.EQ(postgres.UUID(userID)))
	if err != nil {
		return nil, err
	}

	return byUser[userID], nil
}

// ListMethodsByUserIDs returns the methods linked to each of userIDs, keyed by
// user ID and ordered by name ASC within each user, as in ListMethodsByUserID.
// Users with no identities are absent from the map. One query rather than one
// per user, which the admin user list would otherwise pay as an N+1.
func (s *Store) ListMethodsByUserIDs(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID][]entity.AuthMethod, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserIdentities.ListMethodsByUserIDs")
	defer span.End()

	if len(userIDs) == 0 {
		return map[uuid.UUID][]entity.AuthMethod{}, nil
	}

	idExprs := lo.Map(userIDs, func(id uuid.UUID, _ int) postgres.Expression {
		return postgres.UUID(id)
	})

	return s.listMethods(ctx, table.UserIdentities.UserID.IN(idExprs...))
}

// CountProvidersByUserID counts the registry providers userID can sign in
// through -- the same set ListMethodsByUserID reports, so the last-provider
// guard and the UI agree on what "last" means.
//
// Break-glass is not counted. It is the deployment's way in, not a method the
// account holds, and counting it would let a user disconnect the only provider
// they can see and be left with nothing but the emergency path.
func (s *Store) CountProvidersByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserIdentities.CountProvidersByUserID")
	defer span.End()

	stmt := table.UserIdentities.
		SELECT(postgres.COUNT(postgres.STAR).AS("count")).
		WHERE(
			table.UserIdentities.UserID.EQ(postgres.UUID(userID)).
				AND(table.UserIdentities.IntegrationID.IS_NOT_NULL()),
		)

	var dest struct {
		Count int64
	}
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &dest); err != nil {
		return 0, err
	}

	return dest.Count, nil
}

// listMethods returns the sign-in providers of every user matching where, keyed
// by user and sorted by name.
//
// Registry-backed providers only. Break-glass is left out on purpose: it is
// the deployment's way in -- first sign-in on a fresh instance, or an operator
// locked out of the configured providers -- not a method an account links, and
// these lists are what the UI renders as the user's connected sign-in methods.
// The INNER join is what drops it: a built-in identity has no integration_id to
// join on.
//
// The sort is a contract, not presentation: entity.PrimaryAuthMethod takes the
// first element to fill the oauth_provider field, so an unstable order would
// change what a user is told their primary method is.
//
// No dedupe: a user holds at most one identity per registry row (the partial
// unique index), and integration_settings is unique on (kind, name).
func (s *Store) listMethods(ctx context.Context, where postgres.BoolExpression) (map[uuid.UUID][]entity.AuthMethod, error) {
	stmt := postgres.
		SELECT(
			table.UserIdentities.UserID,
			table.IntegrationSettings.Name,
		).
		FROM(table.UserIdentities.
			INNER_JOIN(
				table.IntegrationSettings,
				table.IntegrationSettings.ID.EQ(table.UserIdentities.IntegrationID),
			),
		).
		WHERE(where)

	var rows []*identityMethodRow
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &rows); err != nil {
		return nil, err
	}

	result := make(map[uuid.UUID][]entity.AuthMethod)
	for _, row := range rows {
		result[row.UserID] = append(result[row.UserID], entity.AuthMethod(row.Name))
	}

	for _, methods := range result {
		slices.Sort(methods)
	}

	return result, nil
}
