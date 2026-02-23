package resources

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) GetResourceByID(ctx context.Context, resourceID uuid.UUID) (*entity.ResourceDetails, error) {
	ctx = xlog.WithOperation(ctx, "service.Resources.GetResourceByID")

	resource, err := s.store.GetByID(ctx, resourceID)
	if err != nil {
		xlog.Error(ctx, "failed to get resource", zap.Error(err))
		return nil, err
	}

	return resource, nil
}
