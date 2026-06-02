package maint

import (
	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

func stepsFromCmd(inputs []*entity.MaintenanceStepInput, status entity.MaintenanceStepStatus) []*entity.MaintenanceStep {
	return lo.Map(inputs, func(item *entity.MaintenanceStepInput, _ int) *entity.MaintenanceStep {
		return &entity.MaintenanceStep{
			Order:               item.Order,
			Description:         item.Description,
			RollbackDescription: item.RollbackDescription,
			DurationMinutes:     item.DurationMinutes,
			Status:              status,
		}
	})
}

// deferredNotificationsFromCmd maps contract inputs to domain reminders. Recipients and text
// are not part of a reminder (channels come from the maintenance's notify
// targets, text from the maint.reminder template), so only fire_at is carried.
func deferredNotificationsFromCmd(maintID uuid.UUID, inputs []*entity.DeferredNotificationInput) []*entity.DeferredNotification {
	return lo.Map(inputs, func(input *entity.DeferredNotificationInput, _ int) *entity.DeferredNotification {
		return &entity.DeferredNotification{
			MaintID: maintID,
			FireAt:  input.FireAt,
		}
	})
}
