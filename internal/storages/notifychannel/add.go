package notifychannel

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Store) Add(ctx context.Context, channel *entity.NotifyChannel) (*entity.NotifyChannel, error) {
	_, span := xlog.WithOperationSpan(ctx, "storage.Catalog.Add")
	defer span.End()

	channel, err := s.add(channel)
	if err != nil {
		xlog.Error(ctx, "add channel failed", xfield.Error(err))
		return nil, err
	}

	return channel, nil
}

func (s *Store) add(channel *entity.NotifyChannel) (*entity.NotifyChannel, error) {
	channel.ID = fmt.Sprintf("%s:%s", channel.Transport, channel.TransportChannelID)

	if dup := s.channels.Has(channel.ID); dup {
		return nil, fmt.Errorf("transport '%s' channel already exists: %s", channel.Transport, channel.TransportChannelID)
	}

	s.channels.Set(channel.ID, channel)

	return channel, nil
}
