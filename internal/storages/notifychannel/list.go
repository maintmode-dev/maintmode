package notifychannel

import (
	"context"
	"slices"
	"strings"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Store) List(ctx context.Context) []*entity.NotifyChannel {
	_, span := xlog.WithOperationSpan(ctx, "storage.Catalog.List")
	defer span.End()

	out := s.channels.Values()

	slices.SortFunc(out, func(a, b *entity.NotifyChannel) int {
		return strings.Compare(a.ID, b.ID)
	})

	return out
}
