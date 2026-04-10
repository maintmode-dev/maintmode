package users

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) Update(ctx context.Context, user *entity.User) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Users.Update")
	defer span.End()

	stmt := table.Users.
		UPDATE(table.Users.Roles).
		MODEL(toDBUser(user)).
		WHERE(table.Users.ID.EQ(postgres.UUID(user.ID)))

	_, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return err
	}

	return nil
}
