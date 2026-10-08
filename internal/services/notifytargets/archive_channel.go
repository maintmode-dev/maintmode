package notifytargets

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
)

// ArchiveChannel soft-deletes a channel on behalf of actor: it disappears from
// the default catalog listing but stays resolvable so existing subscriptions
// keep validating. Idempotent: an unknown or already-archived channel is a
// no-op success, and only a real active → archived transition is audited.
func (s *Service) ArchiveChannel(ctx context.Context, actor *entity.User, channelID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Notifytargets.ArchiveChannel")
	defer span.End()

	channel, err := s.setArchived(ctx, channelID, true, s.channelCatalog.Archive)
	if err != nil {
		xlog.Error(ctx, "archive channel failed",
			xfield.String("channelID", channelID.String()),
			xfield.Error(err))
		return err
	}
	if channel != nil {
		s.publishAudit(ctx, audit.NotifyChannelArchived{Actor: actor, Channel: channel})
	}

	return nil
}

// setArchived moves a channel into (archived=true) or out of the archive
// through write, under a row lock so the check and the write are one decision.
// It returns the channel when this call changed its state, and nil when there
// was nothing to do (unknown id, or already in that state).
func (s *Service) setArchived(
	ctx context.Context,
	channelID uuid.UUID,
	archived bool,
	write func(ctx context.Context, channelID uuid.UUID) error,
) (*entity.NotifyChannel, error) {
	var changed *entity.NotifyChannel
	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		channel, err := s.channelCatalog.GetForUpdate(ctx, channelID)
		if errors.Is(err, apperr.ErrNotifyChannelNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if (channel.ArchivedAt != nil) == archived {
			return nil
		}

		if err := write(ctx, channelID); err != nil {
			return err
		}
		changed = channel

		return nil
	})
	if err != nil {
		return nil, err
	}

	return changed, nil
}
