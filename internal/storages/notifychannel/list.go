package notifychannel

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/xsql"
)

// List returns a page of the catalog ordered by (transport,
// transport_channel_id), together with the total number of channels matching the
// same filter. That pair is unique, so the order is total and paging cannot drop
// or duplicate a row between pages.
func (s *Store) List(ctx context.Context, cmd *entity.ListChannelsCmd) ([]*entity.NotifyChannel, int64, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.MessengerChannels.List")
	defer span.End()

	// One predicate value, not two calls to the builder: the count and the page
	// must filter by the same rule for the total to describe the page.
	where := listWhereExpr(cmd)

	total, err := s.countList(ctx, where)
	if err != nil {
		return nil, 0, err
	}

	stmt := table.MessengerChannels.
		SELECT(table.MessengerChannels.AllColumns).
		WHERE(where).
		ORDER_BY(
			table.MessengerChannels.Transport.ASC(),
			table.MessengerChannels.TransportChannelID.ASC(),
		).
		LIMIT(cmd.Limit).
		OFFSET(cmd.Offset)

	result := make([]*model.MessengerChannels, 0, cmd.Limit)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &result); err != nil {
		return nil, 0, err
	}

	return lo.Map(result, func(m *model.MessengerChannels, _ int) *entity.NotifyChannel {
		return fromDB(m)
	}), total, nil
}

func (s *Store) countList(ctx context.Context, where postgres.BoolExpression) (int64, error) {
	stmt := table.MessengerChannels.
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

// listWhereExpr builds the filter shared by the count and the page. The
// always-true seed matters: with no filters active there would otherwise be no
// expression to hand either query.
func listWhereExpr(cmd *entity.ListChannelsCmd) postgres.BoolExpression {
	cond := postgres.Bool(true)

	// Default scope hides soft-deleted channels; IncludeArchived widens it to
	// active + archived.
	if !cmd.IncludeArchived {
		cond = cond.AND(table.MessengerChannels.ArchivedAt.IS_NULL())
	}

	// Optional case-insensitive partial name match (search box on the catalog
	// screen and the maintenance-form picker). EscapeLike keeps % and _ literal
	// so a search for "%" filters instead of matching everything.
	if cmd.Name != "" {
		cond = cond.AND(
			postgres.LOWER(table.MessengerChannels.Name).LIKE(
				postgres.LOWER(postgres.String("%" + xsql.EscapeLike(cmd.Name) + "%"))),
		)
	}

	return cond
}
