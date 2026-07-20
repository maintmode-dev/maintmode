package users

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// ListActiveRoles returns the role set of every non-blocked user — one entry
// per user, in no particular order. It feeds the license seat report: the
// caller buckets each set by its highest role, so the raw sets are returned
// instead of pre-aggregated counts.
func (s *Store) ListActiveRoles(ctx context.Context) ([][]entity.Role, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Users.ListActiveRoles")
	defer span.End()

	stmt := table.Users.
		SELECT(table.Users.Roles).
		WHERE(table.Users.BlockedAt.IS_NULL())

	var rows []model.Users
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &rows); err != nil {
		return nil, err
	}

	return lo.Map(rows, func(r model.Users, _ int) []entity.Role {
		return lo.Map(r.Roles, func(role string, _ int) entity.Role { return entity.Role(role) })
	}), nil
}
