package resources

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) GetResourceTypes(ctx context.Context, resourceID uuid.UUID) ([]entity.ResourceType, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Resources.GetResourceTypes")
	defer span.End()

	_ = ctx
	_ = resourceID

	// For prototype, return all resource type constants
	// In the future, this could filter by types the resource has been used with
	return []entity.ResourceType{
		entity.ResourceTypeService,
		entity.ResourceTypeDatabase,
		entity.ResourceTypeCluster,
	}, nil
}
