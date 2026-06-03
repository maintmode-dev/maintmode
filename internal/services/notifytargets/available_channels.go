package notifytargets

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// AvailableChannels returns the channel catalog. By default archived channels
// are hidden (the picker only shows usable channels); pass includeArchived to
// also return soft-deleted ones (admin/management views).
func (s *Service) AvailableChannels(ctx context.Context, includeArchived bool) ([]*entity.NotifyChannel, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Notifytargets.AvailableChannels")
	defer span.End()

	channels, err := s.channelCatalog.List(ctx, includeArchived)
	if err != nil {
		xlog.Error(ctx, "list channels failed", xfield.Error(err))
		return nil, err
	}

	return channels, nil
}
