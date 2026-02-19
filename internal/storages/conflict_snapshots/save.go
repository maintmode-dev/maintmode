package conflictsnapshots

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) Save(ctx context.Context, maintID uuid.UUID, snaphots []*entity.ConflictWithResources) error {
	ctx = xlog.WithOperation(ctx, "store.ConflictSnapshots.Save")

	if len(snaphots) == 0 {
		return nil
	}

	stmt := table.MaintenanceConflictSnapshot.
		INSERT(table.MaintenanceConflictSnapshot.AllColumns).
		MODELS(toDBSnaphots(maintID, snaphots))

	_, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return err
	}

	return nil
}
