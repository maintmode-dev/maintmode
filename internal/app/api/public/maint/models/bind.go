package apimodels

import (
	"fmt"
	"time"

	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

func FromAPIScope(s MaintenanceScope) (entity.MaintenanceScope, error) {
	switch s {
	case MaintenanceScopeGlobal:
		return entity.MaintenanceScopeGlobal, nil
	case MaintenanceScopeResources:
		return entity.MaintenanceScopeResources, nil
	default:
		return "", fmt.Errorf("unsupported scope: %s", s)
	}
}

func FromAPIImpact(s MaintenanceImpact) (entity.MaintenanceImpact, error) {
	switch s {
	case MaintenanceImpactNone:
		return entity.MaintenanceImpactNone, nil
	case MaintenanceImpactFull:
		return entity.MaintenanceImpactFull, nil
	case MaintenanceImpactPartial:
		return entity.MaintenanceImpactPartial, nil
	default:
		return "", fmt.Errorf("unsupported impact: %s", s)
	}
}

func FromAPIResourceType(s ResourceType) (entity.ResourceType, error) {
	switch s {
	case ResourceTypeDatabase:
		return entity.ResourceTypeDatabase, nil
	case ResourceTypeService:
		return entity.ResourceTypeService, nil
	case ResourceTypeCluster:
		return entity.ResourceTypeCluster, nil
	default:
		return "", fmt.Errorf("unsupported resource type: %s", s)
	}
}

func FromAPIMaintenanceCancelReason(s MaintenanceCancelReason) (entity.MaintenanceCancelReason, error) {
	switch s {
	case MaintenanceCancelReasonConflict:
		return entity.MaintenanceCancelReasonConflict, nil
	case MaintenanceCancelReasonIncident:
		return entity.MaintenanceCancelReasonIncident, nil
	case MaintenanceCancelReasonBusinessDecision:
		return entity.MaintenanceCancelReasonBusinessDecision, nil
	case MaintenanceCancelReasonRescheduled:
		return entity.MaintenanceCancelReasonRescheduled, nil
	case MaintenanceCancelReasonMistake:
		return entity.MaintenanceCancelReasonMistake, nil
	default:
		return "", fmt.Errorf("unsupported cancel reason: %s", s)
	}
}

func FromAPIResources(resources []*Resource) ([]*entity.Resource, error) {
	res := make([]*entity.Resource, 0, len(resources))
	for _, resource := range resources {
		rt, err := FromAPIResourceType(resource.Type)
		if err != nil {
			return nil, err
		}
		res = append(res, &entity.Resource{
			ID:   resource.ID,
			Type: rt,
		})
	}
	return res, nil
}

func ToAPIResources(resources []*entity.Resource) []*Resource {
	return lo.Map(resources, func(item *entity.Resource, _ int) *Resource {
		return &Resource{
			ID:   item.ID,
			Type: ResourceType(item.Type),
		}
	})
}

func ToAPIPeriod(p entity.Period) Period {
	if p.IsOpen() {
		return Period{Start: p.Start, End: nil}
	}
	return Period{Start: p.Start, End: p.End}
}

func FromAPISteps(steps []*MaintenanceStepInput) ([]*entity.MaintenanceStepInput, error) {
	res := make([]*entity.MaintenanceStepInput, 0, len(steps))
	for _, step := range steps {
		duration, err := time.ParseDuration(step.Duration)
		if err != nil {
			return nil, err
		}
		res = append(res, &entity.MaintenanceStepInput{
			Order:               step.Order,
			Description:         step.Description,
			RollbackDescription: step.RollbackDescription,
			DurationMinutes:     int64(duration.Minutes()),
		})
	}

	return res, nil
}

func toAPIStepStatus(s entity.MaintenanceStepStatus) MaintenanceStepStatus {
	switch s {
	case entity.MaintenanceStepStatusPlanned:
		return MaintenanceStepStatusPlanned
	case entity.MaintenanceStepStatusStarted:
		return MaintenanceStepStatusStarted
	case entity.MaintenanceStepStatusCompleted:
		return MaintenanceStepStatusCompleted
	case entity.MaintenanceStepStatusCanceled:
		return MaintenanceStepStatusCanceled
	default:
		return MaintenanceStepStatusUnknown
	}
}

func ToAPISteps(steps []*entity.MaintenanceStep) []*MaintenanceStep {
	return lo.Map(steps, func(item *entity.MaintenanceStep, _ int) *MaintenanceStep {
		return &MaintenanceStep{
			ID:                  item.ID,
			Order:               item.Order,
			Description:         item.Description,
			RollbackDescription: item.RollbackDescription,
			DurationMinutes:     item.DurationMinutes,
			Status:              toAPIStepStatus(item.Status),
		}
	})
}

func ToAPIMaintenance(m *entity.Maintenance) *Maintenance {
	maint := &Maintenance{
		ID:                  m.ID,
		Title:               m.Title,
		Description:         m.Description,
		PlannedPeriod:       ToAPIPeriod(m.PlannedPeriod),
		ActualPeriod:        nil,
		Resources:           ToAPIResources(m.Resources),
		Scope:               MaintenanceScope(m.Scope),
		Impact:              MaintenanceImpact(m.Impact),
		Status:              string(m.Status),
		CancelReason:        MaintenanceCancelReason(m.CancelReason),
		CancelReasonComment: m.CancelReasonComment,
		CreatedAt:           m.CreatedAt,
		UpdatedAt:           m.UpdatedAt,
		Steps:               ToAPISteps(m.Steps),
	}

	if m.ActualPeriod != nil {
		maint.ActualPeriod = lo.ToPtr(ToAPIPeriod(lo.FromPtr(m.ActualPeriod)))
	}

	return maint
}
