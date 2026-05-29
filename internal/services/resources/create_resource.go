package resources

import (
	"context"
	"errors"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) CreateResource(ctx context.Context, cmd *entity.CreateResourceCmd) (*entity.ResourceDetails, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Resources.CreateResource")
	defer span.End()

	resource, err := s.getOrCreate(ctx, cmd)
	if err == nil {
		return resource, nil
	}

	if errors.Is(err, apperr.ErrResourceAlreadyExists) {
		// A concurrent caller won the race past our existence check.
		// The unique index protected us; re-read the winner.
		existing, err := s.GetResourceByName(ctx, cmd.Name)
		if err != nil {
			xlog.Error(ctx, "failed to load existing resource after race",
				xfield.String("name", cmd.Name),
				xfield.Error(err),
			)
			return nil, fmt.Errorf("load existing resource after race: %w", err)
		}
		return existing, nil
	}

	xlog.Error(ctx, "failed to get-or-create resource",
		xfield.String("name", cmd.Name),
		xfield.Error(err),
	)
	return nil, err
}

func (s *Service) getOrCreate(ctx context.Context, cmd *entity.CreateResourceCmd) (*entity.ResourceDetails, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Resources.getOrCreate")
	defer span.End()

	existing, err := s.GetResourceByName(ctx, cmd.Name)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, apperr.ErrResourceNotFound) {
		return nil, fmt.Errorf("check existing resource: %w", err)
	}

	return s.store.Create(ctx, &entity.ResourceDetails{
		Name:        cmd.Name,
		Description: cmd.Description,
		ExternalID:  cmd.ExternalID,
		Status:      entity.ResourceStatusActive,
	})
}
