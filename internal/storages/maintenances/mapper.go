package maintenances

import (
	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
)

func fromDBMaintenance(m *model.Maintenances) *entity.Maintenance {
	maint := &entity.Maintenance{
		ID:                  m.ID,
		Title:               m.Title,
		Description:         m.Description,
		PlannedPeriod:       xtime.FromPgRange(m.PlannedPeriod),
		Scope:               entity.MaintenanceScope(m.Scope),
		Impact:              entity.MaintenanceImpact(m.Impact),
		Status:              entity.MaintenanceStatus(m.Status),
		CancelReason:        entity.MaintenanceCancelReason(lo.FromPtr(m.CanceledReasonCode)),
		CancelReasonComment: lo.FromPtr(m.CanceledReasonComment),
		CreatedAt:           m.CreatedAt,
		UpdatedAt:           m.UpdatedAt,
	}

	if m.ActualPeriod != nil {
		actualPeriod := xtime.FromPgRange(lo.FromPtr(m.ActualPeriod))
		maint.ActualPeriod = &actualPeriod
	}

	return maint
}

func toDBMaintenance(m *entity.Maintenance) *model.Maintenances {
	maint := &model.Maintenances{
		ID:                    m.ID,
		Title:                 m.Title,
		Description:           m.Description,
		PlannedPeriod:         xtime.ToPgRange(m.PlannedPeriod),
		Scope:                 string(m.Scope),
		Impact:                string(m.Impact),
		Status:                string(m.Status),
		CanceledReasonCode:    lo.ToPtr(string(m.CancelReason)),
		CanceledReasonComment: lo.ToPtr(m.CancelReasonComment),
		CreatedAt:             m.CreatedAt,
		UpdatedAt:             m.UpdatedAt,
	}

	if m.ActualPeriod != nil {
		actualPeriod := xtime.ToPgRange(lo.FromPtr(m.ActualPeriod))
		maint.ActualPeriod = &actualPeriod
	}

	return maint
}

func toDBMaintenanceResource(maintID uuid.UUID, resource *entity.Resource) *model.MaintenanceResources {
	return &model.MaintenanceResources{
		MaintenanceID: maintID,
		ResourceID:    resource.ID,
		ResourceType:  string(resource.Type),
	}
}

func fromDBMaintenanceResource(r *model.MaintenanceResources) *entity.Resource {
	return &entity.Resource{
		ID:   r.ResourceID,
		Type: entity.ResourceType(r.ResourceType),
	}
}

func uuidsToPgUUID(resourceIDs []uuid.UUID) []postgres.StringExpression {
	return lo.Map(resourceIDs, func(item uuid.UUID, _ int) postgres.StringExpression {
		return postgres.UUID(item)
	})
}

func toDBMaintenanceStep(maintID uuid.UUID, step *entity.MaintenanceStep) *model.MaintenanceSteps {
	return &model.MaintenanceSteps{
		MaintenanceID:     maintID,
		StepOrder:         step.Order,
		Description:       step.Description,
		Rollback:          step.RollbackDescription,
		DurationMinutes:   step.DurationMinutes,
		Status:            string(step.Status),
		ActualStartedAt:   step.ActualStartedAt,
		ActualCompletedAt: step.ActualCompletedAt,
	}
}

func fromDBMaintenanceStep(step *model.MaintenanceSteps) *entity.MaintenanceStep {
	return &entity.MaintenanceStep{
		ID:                  step.ID,
		Order:               step.StepOrder,
		Description:         step.Description,
		RollbackDescription: step.Rollback,
		DurationMinutes:     step.DurationMinutes,
		Status:              entity.MaintenanceStepStatus(step.Status),
		ActualStartedAt:     step.ActualStartedAt,
		ActualCompletedAt:   step.ActualCompletedAt,
		CreatedAt:           step.CreatedAt,
		UpdatedAt:           step.UpdatedAt,
	}
}
