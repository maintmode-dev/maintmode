package users

import (
	"context"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) Create(ctx context.Context, u *entity.User) (*entity.User, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Users.Create")
	defer span.End()

	user := toDBUser(u)

	stmt := table.Users.
		INSERT(table.Users.MutableColumns.
			Except(table.Users.CreatedAt),
		).
		MODEL(user).
		RETURNING(table.Users.AllColumns)

	err := stmt.QueryContext(ctx, s.db.Executor(ctx), user)
	if err != nil {
		return nil, err
	}

	return fromDBUser(user), nil
}
