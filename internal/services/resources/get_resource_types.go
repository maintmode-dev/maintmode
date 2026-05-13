package resources

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) GetResourceTypes(ctx context.Context, resourceID uuid.UUID) ([]entity.ResourceType, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Resources.GetResourceTypes")
	defer span.End()

	if _, err := s.store.GetByID(ctx, resourceID); err != nil {
		xlog.Error(ctx, "failed to get resource", xfield.Error(err))
		return nil, fmt.Errorf("get resource: %w", err)
	}

	return []entity.ResourceType{
		entity.ResourceTypeService,
		entity.ResourceTypeDatabase,
		entity.ResourceTypeCluster,
	}, nil
}
