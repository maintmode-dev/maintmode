package resources

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// Archive marks a resource as archived. It is idempotent: archiving an
// already-archived or unknown resource succeeds (a non-existent id simply
// updates zero rows).
func (s *Store) Archive(ctx context.Context, resourceID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Resources.Archive")
	defer span.End()

	stmt := table.Resources.
		UPDATE(
			table.Resources.Status,
			table.Resources.UpdatedAt,
		).
		SET(
			postgres.String(string(entity.ResourceStatusArchived)),
			postgres.NOW(),
		).
		WHERE(table.Resources.ID.EQ(postgres.UUID(resourceID)))

	_, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return err
	}

	return nil
}
