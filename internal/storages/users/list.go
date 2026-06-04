package users

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

// List returns a page of users ordered by display name (name) ASC with id ASC as
// a stable tie-breaker, together with the total number of users matching the same
// filter.
func (s *Store) List(ctx context.Context, cmd *entity.ListUsersCmd) ([]*entity.User, int64, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Users.List")
	defer span.End()

	where := listWhereExpr(cmd)

	total, err := s.countList(ctx, where)
	if err != nil {
		return nil, 0, err
	}

	stmt := table.Users.
		SELECT(table.Users.AllColumns).
		WHERE(where).
		ORDER_BY(
			table.Users.Name.ASC(),
			table.Users.ID.ASC(),
		).
		LIMIT(cmd.Limit).
		OFFSET(cmd.Offset)

	users := make([]*model.Users, 0, cmd.Limit)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &users); err != nil {
		return nil, 0, err
	}

	return lo.Map(users, func(r *model.Users, _ int) *entity.User {
		return fromDBUser(r)
	}), total, nil
}

func listWhereExpr(cmd *entity.ListUsersCmd) postgres.BoolExpression {
	cond := postgres.Bool(true)

	// Case-insensitive partial match on display name OR email (search box).
	if cmd.Search != "" {
		pattern := postgres.LOWER(postgres.String("%" + cmd.Search + "%"))
		cond = cond.AND(
			postgres.LOWER(table.Users.Name).LIKE(pattern).
				OR(postgres.LOWER(table.Users.Email).LIKE(pattern)),
		)
	}

	// Optional batch id filter: the author-resolution path restricts to a known
	// set of ids in one query (no N+1 over a page of maintenances).
	if len(cmd.IDs) > 0 {
		ids := lo.Map(cmd.IDs, func(id uuid.UUID, _ int) postgres.Expression {
			return postgres.UUID(id)
		})
		cond = cond.AND(table.Users.ID.IN(ids...))
	}

	// Optional role filter: keep users having ANY of these roles (OR). Expressed
	// as array overlap (roles && ARRAY[...]) — the same && idiom used for period
	// overlap elsewhere — so it is one GIN-indexable expression, not an OR chain.
	if len(cmd.Roles) > 0 {
		roles := lo.Map(cmd.Roles, func(r entity.Role, _ int) string {
			return string(r)
		})
		cond = cond.AND(table.Users.Roles.OVERLAP(postgres.StringArray(roles...)))
	}

	// Assignment pickers hide blocked users; the admin list leaves this off.
	if cmd.ExcludeBlocked {
		cond = cond.AND(table.Users.BlockedAt.IS_NULL())
	}

	return cond
}
