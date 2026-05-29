package notifytargets

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) ResolveNotifyTarget(ctx context.Context, maintID uuid.UUID, input []*entity.NotifyTargetInput) ([]*entity.NotifyTarget, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Notifytargets.ResolveNotifyTarget")
	defer span.End()

	if len(input) == 0 {
		return nil, nil
	}

	notifyTargets := make([]*entity.NotifyTarget, 0, len(input))
	for _, item := range input {
		channel, err := s.channelCatalog.Get(ctx, item.ChannelID)
		if err != nil {
			xlog.Error(ctx, "failed to get channel",
				xfield.String("channelID", item.ChannelID),
				xfield.Error(err))
			return nil, err
		}

		notifyTargets = append(notifyTargets, &entity.NotifyTarget{
			MaintID:   maintID,
			ChannelID: channel.TransportChannelID,
			Transport: channel.Transport,
		})
	}

	return notifyTargets, nil
}
