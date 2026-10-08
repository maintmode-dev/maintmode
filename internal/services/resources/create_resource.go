package resources

import (
	"context"
	"errors"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) CreateResource(ctx context.Context, cmd *entity.CreateResourceCmd) (*entity.ResourceDetails, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Resources.CreateResource")
	defer span.End()

	resource, created, err := s.getOrCreate(ctx, cmd)
	if err == nil {
		// Get-or-create: only a row this call inserted is a catalog change.
		if created {
			s.publishAudit(ctx, audit.ResourceCreated{Actor: cmd.Actor, Resource: resource})
		}
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

// getOrCreate returns the resource named cmd.Name, inserting it when absent;
// created reports whether this call inserted it.
func (s *Service) getOrCreate(ctx context.Context, cmd *entity.CreateResourceCmd) (_ *entity.ResourceDetails, created bool, _ error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Resources.getOrCreate")
	defer span.End()

	existing, err := s.GetResourceByName(ctx, cmd.Name)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, apperr.ErrResourceNotFound) {
		return nil, false, fmt.Errorf("check existing resource: %w", err)
	}

	resource, err := s.store.Create(ctx, &entity.ResourceDetails{
		Name:            cmd.Name,
		Description:     cmd.Description,
		ExternalID:      cmd.ExternalID,
		Status:          entity.ResourceStatusActive,
		CreatedByUserID: &cmd.Actor.ID,
	})
	if err != nil {
		return nil, false, err
	}

	return resource, true, nil
}
