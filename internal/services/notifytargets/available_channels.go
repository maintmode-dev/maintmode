package notifytargets

import (
	"context"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) AvailableChannels(ctx context.Context) ([]*entity.NotifyChannel, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Notifytargets.AvailableChannels")
	defer span.End()

	channels := s.channelCatalog.List(ctx)

	return channels, nil
}
