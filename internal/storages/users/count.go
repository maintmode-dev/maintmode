package users

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// CountActiveAdmins returns the number of non-blocked users holding the admin
// role. It backs the last-admin lockout guard.
func (s *Store) CountActiveAdmins(ctx context.Context) (int64, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Users.CountActiveAdmins")
	defer span.End()

	where := postgres.String(string(entity.RoleAdmin)).EQ(postgres.ANY(table.Users.Roles)).
		AND(table.Users.BlockedAt.IS_NULL())

	return s.countList(ctx, where)
}

func (s *Store) countList(ctx context.Context, where postgres.BoolExpression) (int64, error) {
	stmt := table.Users.
		SELECT(postgres.COUNT(postgres.STAR).AS("count")).
		WHERE(where)

	var dest struct {
		Count int64
	}
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &dest); err != nil {
		return 0, err
	}
	return dest.Count, nil
}
