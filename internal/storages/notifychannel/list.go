package notifychannel

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// List returns the catalog ordered by (transport, transport_channel_id). When
// includeArchived is false (the default for the picker), soft-deleted channels
// (archived_at IS NOT NULL) are filtered out.
func (s *Store) List(ctx context.Context, includeArchived bool) ([]*entity.NotifyChannel, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.MessengerChannels.List")
	defer span.End()

	stmt := table.MessengerChannels.
		SELECT(table.MessengerChannels.AllColumns).
		ORDER_BY(
			table.MessengerChannels.Transport.ASC(),
			table.MessengerChannels.TransportChannelID.ASC(),
		)

	if !includeArchived {
		stmt = stmt.WHERE(table.MessengerChannels.ArchivedAt.IS_NULL())
	}

	result := make([]*model.MessengerChannels, 0)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &result); err != nil {
		return nil, err
	}

	return lo.Map(result, func(m *model.MessengerChannels, _ int) *entity.NotifyChannel {
		return fromDB(m)
	}), nil
}
