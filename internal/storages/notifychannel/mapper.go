package notifychannel

import (
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
)

func toDB(ch *entity.NotifyChannel) *model.MessengerChannels {
	return &model.MessengerChannels{
		Transport:          string(ch.Transport),
		TransportChannelID: ch.TransportChannelID,
		Name:               ch.Name,
		Description:        ch.Description,
	}
}

func fromDB(m *model.MessengerChannels) *entity.NotifyChannel {
	return &entity.NotifyChannel{
		ID:                 m.ID,
		Transport:          entity.NotifyTransport(m.Transport),
		TransportChannelID: m.TransportChannelID,
		Name:               m.Name,
		Description:        m.Description,
		ArchivedAt:         m.ArchivedAt,
	}
}
