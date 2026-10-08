package resources

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
)

// UnarchiveResource restores a resource to active by id on behalf of actor. It
// is idempotent: unarchiving an already-active or unknown resource is a no-op
// success, and only a real archived → active transition is audited.
func (s *Service) UnarchiveResource(ctx context.Context, actor *entity.User, resourceID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Resources.UnarchiveResource")
	defer span.End()

	resource, err := s.setStatus(ctx, resourceID, entity.ResourceStatusActive, s.store.Unarchive)
	if err != nil {
		xlog.Error(ctx, "failed to unarchive resource",
			xfield.String("resource_id", resourceID.String()),
			xfield.Error(err),
		)
		return err
	}
	if resource != nil {
		s.publishAudit(ctx, audit.ResourceUnarchived{Actor: actor, Resource: resource})
	}

	return nil
}
