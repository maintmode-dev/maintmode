package resources

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
)

// UpdateResource applies a partial update to a resource. It loads and locks the
// current row, overlays the provided (non-nil) fields, stamps the editor, and
// persists the result — all in one transaction so the read-modify-write is
// serialized against concurrent writers (e.g. an archive toggling status).
func (s *Service) UpdateResource(ctx context.Context, cmd *entity.UpdateResourceCmd) (*entity.ResourceDetails, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Resources.UpdateResource")
	defer span.End()

	var (
		updated *entity.ResourceDetails
		changes []entity.AuditFieldChange
	)
	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		resource, err := s.store.GetForUpdate(ctx, cmd.ID)
		if err != nil {
			return err
		}

		before := *resource
		applyResourceUpdate(resource, cmd)
		changes = resourceChanges(&before, resource)

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

	s.publishAudit(ctx, audit.ResourceUpdated{Actor: cmd.Actor, Resource: updated, Changes: changes})

	return updated, nil
}

// resourceChanges lists the editable fields an update moved, before and after.
func resourceChanges(before, after *entity.ResourceDetails) []entity.AuditFieldChange {
	var changes []entity.AuditFieldChange
	add := func(field, oldValue, newValue string) {
		if oldValue != newValue {
			changes = append(changes, entity.AuditFieldChange{Field: field, Old: oldValue, New: newValue})
		}
	}
	add("name", before.Name, after.Name)
	add("description", before.Description, after.Description)
	add("external_id", lo.FromPtr(before.ExternalID), lo.FromPtr(after.ExternalID))

	return changes
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

	resource.UpdatedByUserID = &cmd.Actor.ID
}
