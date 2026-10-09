package notifytargets

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
)

// UnarchiveChannel returns a previously archived channel to the active catalog
// on behalf of actor. Idempotent: an unknown or already-active channel is a
// no-op success, and only a real archived → active transition is audited.
func (s *Service) UnarchiveChannel(ctx context.Context, actor *entity.User, channelID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Notifytargets.UnarchiveChannel")
	defer span.End()

	channel, err := s.setArchived(ctx, channelID, false, s.channelCatalog.Unarchive)
	if err != nil {
		xlog.Error(ctx, "unarchive channel failed",
			xfield.String("channelID", channelID.String()),
			xfield.Error(err))
		return err
	}
	if channel != nil {
		s.publishAudit(ctx, audit.NotifyChannelUnarchived{Actor: actor, Channel: channel})
	}

	return nil
}
