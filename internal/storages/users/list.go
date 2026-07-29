package users

import (
	"context"
	"strings"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/xsql"
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

	// Case-insensitive partial match on display name and email (search box);
	// the messenger tags join in only when the caller opted into them.
	if cmd.Search != "" {
		pattern := postgres.LOWER(postgres.String("%" + xsql.EscapeLike(cmd.Search) + "%"))
		match := postgres.LOWER(table.Users.Name).LIKE(pattern).
			OR(postgres.LOWER(table.Users.Email).LIKE(pattern))

		// Gated on SearchMessengerTags: the admin path opts in, the picker does
		// not — see entity.ListUsersCmd for why.
		//
		// Tags are stored verbatim, so "@ruslan" and "ruslan" are different
		// strings (see entity.CanonicalTelegramTag). The "@" is dropped from the
		// query instead: an admin who copied a handle out of a complaint must
		// land on the same row as one who typed the bare name. TrimSpace runs
		// first, since a leading blank would shield the "@" from TrimPrefix —
		// and the paste that carries the "@" carries the blank too.
		//
		// Only the query side needs it. The pattern is unanchored, so a leading
		// "@" left in the column falls inside the opening "%" and matches
		// anyway; stripping the column as well would be a no-op that reads like
		// it does something.
		//
		// The emptiness check is not redundant with the outer one: a query of
		// blanks and/or a bare "@" is non-empty yet trims to nothing, and
		// "handle LIKE '%%'" holds for every non-null handle. That would make
		// the branch always-true and widen the name/email result instead of
		// leaving it alone.
		if tag := strings.TrimPrefix(strings.TrimSpace(cmd.Search), "@"); cmd.SearchMessengerTags && tag != "" {
			tagPattern := postgres.LOWER(postgres.String("%" + xsql.EscapeLike(tag) + "%"))
			match = match.
				OR(postgres.LOWER(table.Users.TelegramTag).LIKE(tagPattern)).
				OR(postgres.LOWER(table.Users.SlackTag).LIKE(tagPattern))
		}

		cond = cond.AND(match)
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
