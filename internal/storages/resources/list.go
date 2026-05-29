package resources

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// List returns a page of resources ordered by created_at DESC (newest first),
// together with the total number of resources matching the same filter.
func (s *Store) List(ctx context.Context, cmd *entity.ListResourcesCmd) ([]*entity.ResourceDetails, int64, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Resources.List")
	defer span.End()

	where := listWhereExpr(cmd)

	total, err := s.countList(ctx, where)
	if err != nil {
		return nil, 0, err
	}

	stmt := table.Resources.
		SELECT(table.Resources.AllColumns).
		WHERE(where).
		ORDER_BY(
			table.Resources.CreatedAt.DESC(),
			table.Resources.ID.DESC(),
		).
		LIMIT(cmd.Limit).
		OFFSET(cmd.Offset)

	resources := make([]*model.Resources, 0, cmd.Limit)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &resources); err != nil {
		return nil, 0, err
	}

	return lo.Map(resources, func(r *model.Resources, _ int) *entity.ResourceDetails {
		return fromDBResource(r)
	}), total, nil
}

func (s *Store) countList(ctx context.Context, where postgres.BoolExpression) (int64, error) {
	stmt := table.Resources.
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

func listWhereExpr(cmd *entity.ListResourcesCmd) postgres.BoolExpression {
	cond := postgres.Bool(true)

	// Default scope hides archived resources; IncludeArchived widens it to all
	// statuses (active + archived).
	if !cmd.IncludeArchived {
		cond = cond.AND(
			table.Resources.Status.EQ(postgres.String(string(entity.ResourceStatusActive))),
		)
	}

	// Optional case-insensitive partial name match (search box on the list page).
	if cmd.Name != "" {
		cond = cond.AND(
			postgres.LOWER(table.Resources.Name).LIKE(
				postgres.LOWER(postgres.String("%" + cmd.Name + "%"))),
		)
	}

	return cond
}
