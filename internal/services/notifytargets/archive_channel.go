package notifytargets

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
)

// ArchiveChannel soft-deletes a channel: it disappears from the default catalog
// listing but stays resolvable so existing subscriptions keep validating.
func (s *Service) ArchiveChannel(ctx context.Context, channelID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Notifytargets.ArchiveChannel")
	defer span.End()

	if err := s.channelCatalog.Archive(ctx, channelID); err != nil {
		xlog.Error(ctx, "archive channel failed",
			xfield.String("channelID", channelID.String()),
			xfield.Error(err))
		return err
	}

	return nil
}
