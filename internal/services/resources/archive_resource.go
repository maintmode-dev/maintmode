package resources

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
)

// ArchiveResource archives a resource by id. It is idempotent: archiving an
// already-archived or unknown resource is a no-op success.
func (s *Service) ArchiveResource(ctx context.Context, resourceID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Resources.ArchiveResource")
	defer span.End()

	if err := s.store.Archive(ctx, resourceID); err != nil {
		xlog.Error(ctx, "failed to archive resource",
			xfield.String("resource_id", resourceID.String()),
			xfield.Error(err),
		)
		return err
	}

	return nil
}
