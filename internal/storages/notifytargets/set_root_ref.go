package notifytargets

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// SetRootRef records the thread root delivered to one channel of a maintenance.
//
// channel is the delivery address the root was actually sent to, and is the
// value the re-pointed-channel guard compares later — see
// entity.NotifyTarget.RootChannel for why the comparison has to be
// address-to-address.
//
// Keyed by the (maintenance_id, channel_id) pair rather than by row id: row ids
// do not survive Replace, which is delete-all + create-all. A target that no
// longer exists updates zero rows and is not an error — the subscription was
// removed between the send and this write, and there is nothing to anchor.
func (s *Store) SetRootRef(
	ctx context.Context,
	maintID, channelID uuid.UUID,
	messageID, channel string,
) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.MaintenanceNotifyTargets.SetRootRef")
	defer span.End()

	upd := model.MaintenanceNotifyTargets{
		RootMessageID: &messageID,
		RootChannel:   &channel,
	}

	stmt := table.MaintenanceNotifyTargets.
		UPDATE(
			table.MaintenanceNotifyTargets.RootMessageID,
			table.MaintenanceNotifyTargets.RootChannel,
		).
		MODEL(upd).
		WHERE(
			table.MaintenanceNotifyTargets.MaintenanceID.EQ(postgres.UUID(maintID)).
				AND(table.MaintenanceNotifyTargets.ChannelID.EQ(postgres.UUID(channelID))),
		)

	if _, err := stmt.ExecContext(ctx, s.db.Executor(ctx)); err != nil {
		return err
	}

	return nil
}
