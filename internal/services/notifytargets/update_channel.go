package notifytargets

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

// UpdateChannel applies a partial update to a channel. It loads the current
// channel, overlays the provided (non-nil) fields, stamps the editor, and
// persists the result. Transport is never touched — it is immutable by design
// (changing it would break notification history and existing subscriptions).
func (s *Service) UpdateChannel(ctx context.Context, cmd *entity.UpdateNotifyChannelCmd) (*entity.NotifyChannel, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Notifytargets.UpdateChannel")
	defer span.End()

	var updated *entity.NotifyChannel
	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		channel, err := s.channelCatalog.GetForUpdate(ctx, cmd.ID)
		if err != nil {
			return err
		}

		applyChannel(channel, cmd)

		updated, err = s.channelCatalog.Update(ctx, channel)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		xlog.Error(ctx, "update channel failed",
			xfield.String("channelID", cmd.ID.String()),
			xfield.Error(err))
		return nil, err
	}

	return updated, nil
}

func applyChannel(channel *entity.NotifyChannel, cmd *entity.UpdateNotifyChannelCmd) {
	if cmd.Name != nil {
		channel.Name = lo.FromPtr(cmd.Name)
	}
	if cmd.Description != nil {
		channel.Description = lo.FromPtr(cmd.Description)
	}
	if cmd.TransportChannelID != nil {
		channel.TransportChannelID = lo.FromPtr(cmd.TransportChannelID)
	}

	channel.UpdatedByUserID = &cmd.UpdatedByUserID
}
