package apimodels

import (
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

type Channel struct {
	ID                 string `json:"id"`
	Transport          string `json:"transport"`
	TransportChannelID string `json:"transport_channel_id"`
	Name               string `json:"name"`
	Description        string `json:"description"`
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
			return &Channel{
				ID:                 item.ID,
				Transport:          string(item.Transport),
				TransportChannelID: item.TransportChannelID,
				Name:               item.Name,
				Description:        item.Description,
			}
		}),
	}
}
