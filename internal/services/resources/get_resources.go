package resources

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) GetResources(ctx context.Context, resourceIDs []uuid.UUID) ([]*entity.ResourceDetails, error) {
	ctx = xlog.WithOperation(ctx, "service.Resources.GetResources")

	resources, err := s.store.GetResources(ctx, resourceIDs)
	if err != nil {
		xlog.Error(ctx, "failed to search resources", zap.Error(err))
		return nil, err
	}

	return resources, nil
}

func (s *Service) GetResourcesLikeName(ctx context.Context, name string) ([]*entity.ResourceDetails, error) {
	ctx = xlog.WithOperation(ctx, "service.Resources.GetResourcesLikeName")

	resources, err := s.store.GetResourcesLikeName(ctx, name)
	if err != nil {
		xlog.Error(ctx, "failed to search resources", zap.Error(err))
		return nil, err
	}

	return resources, nil
}
