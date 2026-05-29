package resources

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
)

// UnarchiveResource restores a resource to active by id. It is idempotent:
// unarchiving an already-active or unknown resource is a no-op success.
func (s *Service) UnarchiveResource(ctx context.Context, resourceID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Resources.UnarchiveResource")
	defer span.End()

	if err := s.store.Unarchive(ctx, resourceID); err != nil {
		xlog.Error(ctx, "failed to unarchive resource",
			xfield.String("resource_id", resourceID.String()),
			xfield.Error(err),
		)
		return err
	}

	return nil
}
