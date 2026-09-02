// Package apinotifications exposes HTTP handlers for the
// notifications domain. Today it serves the channel catalog used by
// the admin UI's channel picker.
package apinotifications

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/gateways/notifytransport"
	"github.com/ruko1202/maintmode/internal/services/usersummary"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/notifytargets/models"
)

// transportSource resolves a delivery transport by name — the SAME seam the
// send paths use (transportresolver.Service, or the stub in dev), so
// transport_status is derived from exactly what a real send would do:
// resolves → ok, and the error's sentinel tells disabled / not_configured /
// unreadable apart. The resolved transport itself is discarded by
// this layer; only the outcome is read. Secrets never surface here — they stay
// inside the built client.
type transportSource interface {
	Get(ctx context.Context, name entity.NotifyTransport) (notifytransport.Transport, error)
}

// channelService is the consumer-side port to the channel-catalog service —
// exactly the calls the handlers make, so unit tests can substitute a
// recording fake (error propagation, index-before-write ordering) without a
// database. Satisfied by *notifytargets.Service.
type channelService interface {
	AvailableChannels(ctx context.Context, cmd *entity.ListChannelsCmd) (*entity.ListChannelsResult, error)
	GetChannel(ctx context.Context, channelID uuid.UUID) (*entity.NotifyChannel, error)
	CreateChannel(ctx context.Context, cmd *entity.CreateNotifyChannelCmd) (*entity.NotifyChannel, error)
	UpdateChannel(ctx context.Context, cmd *entity.UpdateNotifyChannelCmd) (*entity.NotifyChannel, error)
	ArchiveChannel(ctx context.Context, channelID uuid.UUID) error
	UnarchiveChannel(ctx context.Context, channelID uuid.UUID) error
}

type Implementation struct {
	notifyTargets  channelService
	userSummarySrv *usersummary.Service
	transports     transportSource
}

func New(
	notifyTargets channelService,
	userSummarySrv *usersummary.Service,
	transports transportSource,
) *Implementation {
	return &Implementation{
		notifyTargets:  notifyTargets,
		userSummarySrv: userSummarySrv,
		transports:     transports,
	}
}

// transportStatuses resolves each of the given transports once (duplicates
// collapse) and classifies the outcome into the status index. The resolver's
// own cache makes the warm path a map lookup per transport. An unclassifiable
// resolve failure (e.g. storage down) fails the request loudly (500) —
// transport_status is a mandatory read-model field, and a made-up status would
// hide an outage. The failure is logged here once for all handlers.
func (i *Implementation) transportStatuses(ctx context.Context, transports []entity.NotifyTransport) (apimodels.TransportStatusIndex, error) {
	index := make(apimodels.TransportStatusIndex, len(transports))
	for _, transport := range transports {
		if _, done := index[transport]; done {
			continue
		}
		_, err := i.transports.Get(ctx, transport)

		status, ok := apimodels.StatusFromResolve(err)
		if !ok {
			xlog.Error(ctx, "resolve transport status failed",
				xfield.String("transport", string(transport)),
				xfield.Error(err))
			return nil, err
		}
		index[transport] = status
	}
	return index, nil
}
