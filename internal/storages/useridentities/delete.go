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
