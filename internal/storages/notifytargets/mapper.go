package notifytargets

import (
	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
)

func toDB(maintID uuid.UUID, sub *entity.NotifyTarget) *model.MaintenanceNotifyTargets {
	return &model.MaintenanceNotifyTargets{
		MaintenanceID: maintID,
		Transport:     string(sub.Transport),
		ChannelID:     sub.ChannelID,
	}
}

func fromDB(m *model.MaintenanceNotifyTargets) *entity.NotifyTarget {
	return &entity.NotifyTarget{
		ID:        m.ID,
		MaintID:   m.MaintenanceID,
		Transport: entity.NotifyTransport(m.Transport),
		ChannelID: m.ChannelID,
		CreatedAt: m.CreatedAt,
	}
}
