package deferrednotifications

import (
	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
)

func toDB(maintID uuid.UUID, n *entity.DeferredNotification) *model.MaintenanceDeferredNotifications {
	return &model.MaintenanceDeferredNotifications{
		MaintenanceID: maintID,
		FireAt:        n.FireAt,
		GoqueTaskID:   n.GoqueTaskID,
	}
}

func fromDB(m *model.MaintenanceDeferredNotifications) *entity.DeferredNotification {
	return &entity.DeferredNotification{
		ID:          m.ID,
		MaintID:     m.MaintenanceID,
		FireAt:      m.FireAt,
		GoqueTaskID: m.GoqueTaskID,
	}
}
