package notifytargets

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

// listRow is the join destination. The embedded model soaks up the target's
// own columns; the channel's catalog data arrives under the "channel.*"
// aliases, which qrm matches via the `alias` struct tag (not `db`). The FK
// guarantees a match, so the join is INNER and the fields are non-pointers.
type listRow struct {
	model.MaintenanceNotifyTargets
	ChannelTransport          string `alias:"channel.transport"`
	ChannelTransportChannelID string `alias:"channel.transport_channel_id"`
	ChannelName               string `alias:"channel.name"`
}

// ListByMaint returns the maintenance's notify targets with their catalog
// channel resolved (transport, delivery address, name). The address is read
// from the catalog at call time — editing a channel redirects notifications.
// Archived channels resolve on purpose: archiving hides a channel from the
// default listing but does not unsubscribe existing maintenances.
func (s *Store) ListByMaint(ctx context.Context, maintID uuid.UUID) ([]*entity.NotifyTarget, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.MaintenanceNotifyTargets.ListByMaint")
	defer span.End()

	stmt := table.MaintenanceNotifyTargets.
		INNER_JOIN(table.MessengerChannels,
			table.MessengerChannels.ID.EQ(table.MaintenanceNotifyTargets.ChannelID),
		).
		SELECT(
			table.MaintenanceNotifyTargets.AllColumns,
			table.MessengerChannels.Transport.AS("channel.transport"),
			table.MessengerChannels.TransportChannelID.AS("channel.transport_channel_id"),
			table.MessengerChannels.Name.AS("channel.name"),
		).
		WHERE(table.MaintenanceNotifyTargets.MaintenanceID.EQ(postgres.UUID(maintID))).
		ORDER_BY(
			table.MaintenanceNotifyTargets.CreatedAt.ASC(),
			table.MaintenanceNotifyTargets.ChannelID.ASC(),
		)

	rows := make([]*listRow, 0)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &rows); err != nil {
		return nil, err
	}

	return lo.Map(rows, func(r *listRow, _ int) *entity.NotifyTarget {
		m := r.MaintenanceNotifyTargets
		target := fromDB(&m)
		target.Transport = entity.NotifyTransport(r.ChannelTransport)
		target.TransportChannelID = r.ChannelTransportChannelID
		target.ChannelName = r.ChannelName
		return target
	}), nil
}
