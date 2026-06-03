package notifytargets

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
)

// UnarchiveChannel returns a previously archived channel to the active catalog.
func (s *Service) UnarchiveChannel(ctx context.Context, channelID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Notifytargets.UnarchiveChannel")
	defer span.End()

	if err := s.channelCatalog.Unarchive(ctx, channelID); err != nil {
		xlog.Error(ctx, "unarchive channel failed",
			xfield.String("channelID", channelID.String()),
			xfield.Error(err))
		return err
	}

	return nil
}
