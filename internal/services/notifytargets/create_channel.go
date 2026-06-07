package notifytargets

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// CreateChannel registers a new channel in the catalog. The catalog is the
// single source of truth for subscription validation, so a channel created here
// becomes immediately usable across all instances.
func (s *Service) CreateChannel(ctx context.Context, cmd *entity.CreateNotifyChannelCmd) (*entity.NotifyChannel, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Notifytargets.CreateChannel")
	defer span.End()

	if !cmd.Transport.IsValid() {
		return nil, fmt.Errorf("%w: unsupported transport %q", apperr.ErrValidation, cmd.Transport)
	}

	channel, err := s.channelCatalog.Create(ctx, &entity.NotifyChannel{
		Transport:          cmd.Transport,
		TransportChannelID: cmd.TransportChannelID,
		Name:               cmd.Name,
		Description:        cmd.Description,
		CreatedByUserID:    &cmd.CreatedByUserID,
	})
	if err != nil {
		xlog.Error(ctx, "create channel failed", xfield.Error(err))
		return nil, err
	}

	return channel, nil
}
