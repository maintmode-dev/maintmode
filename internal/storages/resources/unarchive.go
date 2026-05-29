package resources

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// Unarchive marks a resource as active. It is idempotent: unarchiving an
// already-active or unknown resource succeeds (a non-existent id simply
// updates zero rows).
func (s *Store) Unarchive(ctx context.Context, resourceID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Resources.Unarchive")
	defer span.End()

	stmt := table.Resources.
		UPDATE(
			table.Resources.Status,
			table.Resources.UpdatedAt,
		).
		SET(
			postgres.String(string(entity.ResourceStatusActive)),
			postgres.NOW(),
		).
		WHERE(table.Resources.ID.EQ(postgres.UUID(resourceID)))

	_, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return err
	}

	return nil
}
