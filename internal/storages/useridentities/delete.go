package useridentities

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// DeleteUserIdentityByProviderName removes one user's identity with a login
// provider, addressed by the provider's NAME.
//
// The name is what arrives: the endpoint is /me/providers/{provider}/disconnect,
// and {provider} is a name, never an id. Resolving it to the row id first and
// deleting second would be two round trips for one statement's worth of work,
// so the join does it in place.
//
// It joins integration_settings rather than consulting the registry service,
// and that is the point: unlinking is how a user escapes a provider that has
// been REMOVED from configuration. A lookup that started at the registry would
// fail for exactly the identities most in need of removal. Here the row is
// found among the user's own, and the registry row supplies nothing but its
// name.
//
// Built-in methods are not reachable through this path -- DisconnectProvider
// refuses them before any of this -- so there is no builtin_method branch to
// handle. The join enforces that on its own: a built-in row has no
// integration_id to join on.
//
// Deleting an identity the user does not hold is a no-op, which is the
// idempotence /disconnect promises: "you are not linked to this" and "stop
// linking me to this" describe the same desired state.
func (s *Store) DeleteUserIdentityByProviderName(
	ctx context.Context, userID uuid.UUID, name entity.AuthMethod,
) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserIdentities.DeleteUserIdentityByProviderName")
	defer span.End()

	stmt := table.UserIdentities.
		DELETE().
		USING(table.IntegrationSettings).
		WHERE(
			table.UserIdentities.UserID.EQ(postgres.UUID(userID)).
				AND(table.IntegrationSettings.ID.EQ(table.UserIdentities.IntegrationID)).
				AND(table.IntegrationSettings.Name.EQ(postgres.String(string(name)))),
		)

	if _, err := stmt.ExecContext(ctx, s.db.Executor(ctx)); err != nil {
		return err
	}

	return nil
}

// DeleteByIntegrationID removes every identity authenticating through one
// registry row, reporting how many went so the caller can tell an operator what
// the delete cost.
//
// It is the cascade the integration service performs before removing a provider.
// The database will not do it: the foreign key is ON DELETE RESTRICT precisely
// so that removing a provider's accounts stays an explicit act, visible at the
// call site and observable by a test when it goes missing.
//
// Addressed by id rather than by name, which is the whole point of the column:
// a name could be re-created against a different IdP, an id cannot.
func (s *Store) DeleteByIntegrationID(ctx context.Context, integrationID uuid.UUID) (int64, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserIdentities.DeleteByIntegrationID")
	defer span.End()

	stmt := table.UserIdentities.
		DELETE().
		WHERE(table.UserIdentities.IntegrationID.EQ(postgres.UUID(integrationID)))

	res, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}
