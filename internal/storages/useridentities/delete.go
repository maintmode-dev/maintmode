package useridentities

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// DeleteByUserAndProvider removes the identity linking userID to provider.
// Deleting a non-existent identity is a no-op (idempotent): only a real DB
// failure returns an error.
func (s *Store) DeleteByUserAndProvider(ctx context.Context, userID uuid.UUID, provider entity.AuthMethod) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserIdentities.DeleteByUserAndProvider")
	defer span.End()

	stmt := table.UserIdentities.
		DELETE().
		WHERE(
			table.UserIdentities.UserID.EQ(postgres.UUID(userID)).
				AND(table.UserIdentities.Provider.EQ(postgres.String(string(provider)))),
		)

	if _, err := stmt.ExecContext(ctx, s.db.Executor(ctx)); err != nil {
		return err
	}

	return nil
}

// DeleteByProvider removes every identity linked to one provider name and
// reports how many went, so the caller can tell an operator what the delete
// cost.
//
// It exists for the cascade in the integration service: deleting a login
// provider used to be refused while accounts still signed in through it, and
// the refusal pointed at an unlink that only the account's OWNER can perform,
// through /me. An admin removing a provider had no way to satisfy it short of
// editing the database.
//
// Leaving the rows behind instead would be worse than either. provider is a
// bare string with no foreign key, so an orphan row is not inert: it waits for
// someone to re-create the name against another IdP, and then vouches for that
// IdP against an account that predates it.
func (s *Store) DeleteByProvider(ctx context.Context, provider entity.AuthMethod) (int64, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserIdentities.DeleteByProvider")
	defer span.End()

	stmt := table.UserIdentities.
		DELETE().
		WHERE(table.UserIdentities.Provider.EQ(postgres.String(string(provider))))

	res, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}
