package resources

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) GetResourceByID(ctx context.Context, resourceID uuid.UUID) (*entity.ResourceDetails, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Resources.GetResourceByID")
	defer span.End()

	resource, err := s.store.GetByID(ctx, resourceID)
	if err != nil {
		xlog.Error(ctx, "failed to get resource", xfield.Error(err))
		return nil, err
	}

	return resource, nil
}
