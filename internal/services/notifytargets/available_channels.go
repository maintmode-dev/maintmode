package notifytargets

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// AvailableChannels returns a page of the channel catalog plus the total number
// of channels matching the same filter. By default archived channels are hidden
// (the picker only shows usable ones); cmd.IncludeArchived widens the scope to
// soft-deleted ones as well (admin/management views).
func (s *Service) AvailableChannels(ctx context.Context, cmd *entity.ListChannelsCmd) (*entity.ListChannelsResult, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Notifytargets.AvailableChannels")
	defer span.End()

	channels, total, err := s.channelCatalog.List(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "list channels failed", xfield.Error(err))
		return nil, err
	}

	return &entity.ListChannelsResult{
		Channels: channels,
		Total:    total,
	}, nil
}
