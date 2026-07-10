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
	"github.com/ruko1202/maintmode/internal/services/usersummary"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/notifytargets/models"
)

// integrationSource is the read-only view of the integration registry the
// channel read models derive transport_status from (RUK-198). The masked
// listing never carries secrets, so this is the only registry surface the
// notifications API is allowed to see. Implemented by integration.Service.
type integrationSource interface {
	List(ctx context.Context) ([]*entity.MaskedIntegration, error)
}

// channelService is the consumer-side port to the channel-catalog service —
// exactly the calls the handlers make, so unit tests can substitute a
// recording fake (error propagation, index-before-write ordering) without a
// database. Satisfied by *notifytargets.Service.
type channelService interface {
	AvailableChannels(ctx context.Context, includeArchived bool) ([]*entity.NotifyChannel, error)
	GetChannel(ctx context.Context, channelID uuid.UUID) (*entity.NotifyChannel, error)
	CreateChannel(ctx context.Context, cmd *entity.CreateNotifyChannelCmd) (*entity.NotifyChannel, error)
	UpdateChannel(ctx context.Context, cmd *entity.UpdateNotifyChannelCmd) (*entity.NotifyChannel, error)
	ArchiveChannel(ctx context.Context, channelID uuid.UUID) error
	UnarchiveChannel(ctx context.Context, channelID uuid.UUID) error
}

type Implementation struct {
	notifyTargets  channelService
	userSummarySrv *usersummary.Service
	integrations   integrationSource
}

func New(
	notifyTargets channelService,
	userSummarySrv *usersummary.Service,
	integrations integrationSource,
) *Implementation {
	return &Implementation{
		notifyTargets:  notifyTargets,
		userSummarySrv: userSummarySrv,
		integrations:   integrations,
	}
}

// integrationIndex snapshots the registry into the kind→enabled view, one List
// call per request. A registry read failure fails the request loudly (500) —
// transport_status is a mandatory read-model field, not a best-effort hint.
// The failure is logged here once for all handlers (the service layer logs its
// own storage detail).
func (i *Implementation) integrationIndex(ctx context.Context) (apimodels.IntegrationIndex, error) {
	integrations, err := i.integrations.List(ctx)
	if err != nil {
		xlog.Error(ctx, "build integration index failed", xfield.Error(err))
		return nil, err
	}
	return apimodels.NewIntegrationIndex(integrations), nil
}
