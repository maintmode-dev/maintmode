package resources

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// UpdateResource applies a partial update to a resource. It loads the current
// resource, overlays the provided (non-nil) fields, and persists the result.
func (s *Service) UpdateResource(ctx context.Context, cmd *entity.UpdateResourceCmd) (*entity.ResourceDetails, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Resources.UpdateResource")
	defer span.End()

	resource, err := s.GetResourceByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	if cmd.Name != nil {
		resource.Name = *cmd.Name
	}
	if cmd.Description != nil {
		resource.Description = *cmd.Description
	}
	if cmd.ExternalID != nil {
		resource.ExternalID = cmd.ExternalID
	}

	updated, err := s.store.Update(ctx, resource)
	if err != nil {
		xlog.Error(ctx, "failed to update resource",
			xfield.String("resource_id", cmd.ID.String()),
			xfield.Error(err),
		)
		return nil, err
	}

	return updated, nil
}
