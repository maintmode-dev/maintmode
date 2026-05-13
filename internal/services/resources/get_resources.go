package resources

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) GetResources(ctx context.Context, resourceIDs []uuid.UUID) ([]*entity.ResourceDetails, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Resources.GetResources")
	defer span.End()

	resources, err := s.store.GetResources(ctx, resourceIDs)
	if err != nil {
		xlog.Error(ctx, "failed to search resources", xfield.Error(err))
		return nil, err
	}

	return resources, nil
}

func (s *Service) GetResourcesLikeName(ctx context.Context, name string) ([]*entity.ResourceDetails, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Resources.GetResourcesLikeName")
	defer span.End()

	resources, err := s.store.GetResourcesLikeName(ctx, name)
	if err != nil {
		xlog.Error(ctx, "failed to search resources", xfield.Error(err))
		return nil, err
	}

	return resources, nil
}

func (s *Service) GetResourceByName(ctx context.Context, name string) (*entity.ResourceDetails, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Resources.GetResourceByName")
	defer span.End()

	resource, err := s.store.GetByName(ctx, name)
	if err != nil {
		// ErrResourceNotFound is an expected branch (e.g. CreateResource probes
		// for an existing record). Let the caller decide whether to log it.
		if !errors.Is(err, apperr.ErrResourceNotFound) {
			xlog.Error(ctx, "failed to get resource", xfield.String("name", name), xfield.Error(err))
		}
		return nil, err
	}

	return resource, nil
}
