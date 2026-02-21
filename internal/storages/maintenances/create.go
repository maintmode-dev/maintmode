package maintenances

import (
	"context"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) Create(ctx context.Context, m *entity.Maintenance) error {
	ctx = xlog.WithOperation(ctx, "store.Maintenances.Create")

	maint := toDBMaintenance(m)

	stmt := table.Maintenances.
		INSERT(table.Maintenances.AllColumns).
		MODEL(maint)

	_, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return err
	}

	return nil
}
