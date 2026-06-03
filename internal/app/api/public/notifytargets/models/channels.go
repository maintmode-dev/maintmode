package apimodels

import (
	"time"

	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

type Channel struct {
	ID                 string     `json:"id" format:"uuid"`
	Transport          string     `json:"transport"`
	TransportChannelID string     `json:"transport_channel_id"`
	Name               string     `json:"name"`
	Description        string     `json:"description"`
	ArchivedAt         *time.Time `json:"archived_at,omitempty" format:"date-time"`
}

// CreateChannelRequest is the body of POST /api/v1/notifications/channels.
// The id (a UUID) is assigned by the DB and returned in the response, so it is
// not part of the request.
type CreateChannelRequest struct {
	Transport          string `json:"transport" example:"slack"`
	TransportChannelID string `json:"transport_channel_id" example:"C0123456789"`
	Name               string `json:"name" example:"Alerts"`
	Description        string `json:"description" example:"Ops alerting channel"`
}

func ToChannel(channel *entity.NotifyChannel) *Channel {
	return &Channel{
		ID:                 channel.ID.String(),
		Transport:          string(channel.Transport),
		TransportChannelID: channel.TransportChannelID,
		Name:               channel.Name,
		Description:        channel.Description,
		ArchivedAt:         channel.ArchivedAt,
	}
}

// ChannelsResponse is the payload of GET /api/v1/notifications/channels.
// Wrapping the slice in an object leaves room for future fields
// (pagination, transport metadata) without a breaking change.
type ChannelsResponse struct {
	Channels []*Channel `json:"channels"`
}

func ToChannelsResponse(channels []*entity.NotifyChannel) ChannelsResponse {
	return ChannelsResponse{
		Channels: lo.Map(channels, func(item *entity.NotifyChannel, _ int) *Channel {
			return ToChannel(item)
		}),
	}
}
