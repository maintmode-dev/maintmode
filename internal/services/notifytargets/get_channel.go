package notifytargets

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// GetChannel resolves a single channel by its catalog id. Archived channels are
// returned too: the detail page must render an archived channel.
func (s *Service) GetChannel(ctx context.Context, channelID uuid.UUID) (*entity.NotifyChannel, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Notifytargets.GetChannel")
	defer span.End()

	channel, err := s.channelCatalog.Get(ctx, channelID)
	if err != nil {
		xlog.Error(ctx, "get channel failed",
			xfield.String("channelID", channelID.String()),
			xfield.Error(err))
		return nil, err
	}

	return channel, nil
}
