package notifychannel

import (
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
)

func toDB(ch *entity.NotifyChannel) *model.MessengerChannels {
	return &model.MessengerChannels{
		ID:                 ch.ID,
		Transport:          string(ch.Transport),
		TransportChannelID: ch.TransportChannelID,
		Name:               ch.Name,
		Description:        ch.Description,
		CreatedByUserID:    ch.CreatedByUserID,
		UpdatedAt:          ch.UpdatedAt,
		UpdatedByUserID:    ch.UpdatedByUserID,
	}
}

func fromDB(m *model.MessengerChannels) *entity.NotifyChannel {
	return &entity.NotifyChannel{
		ID:                 m.ID,
		Transport:          entity.NotifyTransport(m.Transport),
		TransportChannelID: m.TransportChannelID,
		Name:               m.Name,
		Description:        m.Description,
		CreatedAt:          m.CreatedAt,
		CreatedByUserID:    m.CreatedByUserID,
		UpdatedAt:          m.UpdatedAt,
		UpdatedByUserID:    m.UpdatedByUserID,
		ArchivedAt:         m.ArchivedAt,
	}
}
