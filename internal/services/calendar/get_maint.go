package calendar

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/calendardto"
)

func (s *Service) GetMaint(ctx context.Context, maintID uuid.UUID) (*calendardto.Maintenance, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Calendar.GetMaint")
	defer span.End()

	maint, err := s.maintStore.GetMaint(ctx, maintID)
	if err != nil {
		xlog.Error(ctx, "failed to get maint", xfield.Error(err))
		return nil, err
	}

	maintResourcesM, err := s.getMaintResources(ctx, []uuid.UUID{maint.ID})
	if err != nil {
		xlog.Error(ctx, "failed to get maint resources", xfield.Error(err))
		return nil, err
	}

	steps, err := s.maintStore.GetMaintSteps(ctx, maint.ID)
	if err != nil {
		xlog.Error(ctx, "failed to get maint steps", xfield.Error(err))
		return nil, err
	}

	return &calendardto.Maintenance{
		ID:                  maint.ID,
		Title:               maint.Title,
		Description:         maint.Description,
		PlannedPeriod:       maint.PlannedPeriod,
		ActualPeriod:        maint.ActualPeriod,
		Resources:           lo.ValueOr(maintResourcesM, maint.ID, []*calendardto.MaintenanceResource{}),
		Scope:               maint.Scope,
		Impact:              maint.Impact,
		Status:              maint.Status,
		CancelReason:        maint.CancelReason,
		CancelReasonComment: maint.CancelReasonComment,
		CreatedAt:           maint.CreatedAt,
		UpdatedAt:           maint.UpdatedAt,
		Revision:            maint.Revision(),
		Steps: lo.Map(steps, func(item *entity.MaintenanceStep, _ int) *calendardto.MaintenanceStep {
			return &calendardto.MaintenanceStep{
				ID:                  item.ID,
				Order:               item.Order,
				Description:         item.Description,
				RollbackDescription: item.RollbackDescription,
				DurationMinutes:     item.DurationMinutes,
				Status:              item.Status,
			}
		}),
	}, nil
}

func (s *Service) getMaintResources(ctx context.Context, maintIDs []uuid.UUID) (map[uuid.UUID][]*calendardto.MaintenanceResource, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Calendar.GetMaintResources")
	defer span.End()

	if len(maintIDs) == 0 {
		return map[uuid.UUID][]*calendardto.MaintenanceResource{}, nil
	}

	maintsResources, err := s.maintStore.GetMaintResources(ctx, maintIDs)
	if err != nil {
		return nil, err
	}

	resourcesDetails, err := s.getResourcesDetails(ctx, lo.Map(
		lo.Flatten(lo.Values(maintsResources)), func(item *entity.Resource, _ int) uuid.UUID {
			return item.ID
		}),
	)
	if err != nil {
		return nil, err
	}

	maintResourcesM := make(map[uuid.UUID][]*calendardto.MaintenanceResource)
	for maintID, resources := range maintsResources {
		eventResources := lo.Map(resources, func(item *entity.Resource, _ int) *calendardto.MaintenanceResource {
			details := lo.ValueOr(resourcesDetails, item.ID, &entity.ResourceDetails{Name: "unknown resource"})
			return &calendardto.MaintenanceResource{
				ID:   item.ID,
				Name: details.Name,
				Type: item.Type,
			}
		})

		maintResourcesM[maintID] = append(maintResourcesM[maintID], eventResources...)
	}

	return maintResourcesM, nil
}

func (s *Service) getResourcesDetails(ctx context.Context, resources []uuid.UUID) (map[uuid.UUID]*entity.ResourceDetails, error) {
	resourcesDetails, err := s.resourcesStore.GetResources(ctx, resources)
	if err != nil {
		return nil, err
	}

	return lo.SliceToMap(resourcesDetails, func(item *entity.ResourceDetails) (uuid.UUID, *entity.ResourceDetails) {
		return item.ID, item
	}), nil
}
