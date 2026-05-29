package notifychannel

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Store) Get(ctx context.Context, channelID string) (*entity.NotifyChannel, error) {
	_, span := xlog.WithOperationSpan(ctx, "storage.Catalog.Get")
	defer span.End()

	ch, ok := s.channels.Get(channelID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", apperr.ErrNotFound, channelID)
	}

	return ch, nil
}
