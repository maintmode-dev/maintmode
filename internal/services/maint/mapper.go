package maint

import (
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
