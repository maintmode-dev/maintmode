package resources

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
)

// ArchiveResource archives a resource by id on behalf of actor. It is
// idempotent: archiving an already-archived or unknown resource is a no-op
// success, and only a real active → archived transition is audited.
func (s *Service) ArchiveResource(ctx context.Context, actor *entity.User, resourceID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Resources.ArchiveResource")
	defer span.End()

	resource, err := s.setStatus(ctx, resourceID, entity.ResourceStatusArchived, s.store.Archive)
	if err != nil {
		xlog.Error(ctx, "failed to archive resource",
			xfield.String("resource_id", resourceID.String()),
			xfield.Error(err),
		)
		return err
	}
	if resource != nil {
		s.publishAudit(ctx, audit.ResourceArchived{Actor: actor, Resource: resource})
	}

	return nil
}

// setStatus moves a resource to status through write, under a row lock so the
// check and the write are one decision. It returns the resource when this call
// changed its status, and nil when there was nothing to do (unknown id, or
// already in that status) -- the idempotent no-op the endpoints promise.
func (s *Service) setStatus(
	ctx context.Context,
	resourceID uuid.UUID,
	status entity.ResourceStatus,
	write func(ctx context.Context, resourceID uuid.UUID) error,
) (*entity.ResourceDetails, error) {
	var changed *entity.ResourceDetails
	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		resource, err := s.store.GetForUpdate(ctx, resourceID)
		if errors.Is(err, apperr.ErrResourceNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if resource.Status == status {
			return nil
		}

		if err := write(ctx, resourceID); err != nil {
			return err
		}
		resource.Status = status
		changed = resource

		return nil
	})
	if err != nil {
		return nil, err
	}

	return changed, nil
}
