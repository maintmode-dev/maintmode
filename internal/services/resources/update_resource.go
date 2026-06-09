package resources

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// UpdateResource applies a partial update to a resource. It loads and locks the
// current row, overlays the provided (non-nil) fields, stamps the editor, and
// persists the result — all in one transaction so the read-modify-write is
// serialized against concurrent writers (e.g. an archive toggling status).
func (s *Service) UpdateResource(ctx context.Context, cmd *entity.UpdateResourceCmd) (*entity.ResourceDetails, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Resources.UpdateResource")
	defer span.End()

	var updated *entity.ResourceDetails
	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		resource, err := s.store.GetForUpdate(ctx, cmd.ID)
		if err != nil {
			return err
		}

		applyResourceUpdate(resource, cmd)

		updated, err = s.store.Update(ctx, resource)
		return err
	})
	if err != nil {
		xlog.Error(ctx, "failed to update resource",
			xfield.String("resource_id", cmd.ID.String()),
			xfield.Error(err),
		)
		return nil, err
	}

	return updated, nil
}

// applyResourceUpdate overlays the non-nil command fields onto the loaded
// resource and stamps the editor.
func applyResourceUpdate(resource *entity.ResourceDetails, cmd *entity.UpdateResourceCmd) {
	if cmd.Name != nil {
		resource.Name = *cmd.Name
	}
	if cmd.Description != nil {
		resource.Description = *cmd.Description
	}
	if cmd.ExternalID != nil {
		resource.ExternalID = cmd.ExternalID
	}

	resource.UpdatedByUserID = &cmd.UpdatedByUserID
}
