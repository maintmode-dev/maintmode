package maintenances

import (
	"context"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) CreateMaint(ctx context.Context, m *entity.Maintenance) (*entity.Maintenance, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Maintenances.CreateMaint")
	defer span.End()

	maint := toDBMaintenance(m)

	stmt := table.Maintenances.
		INSERT(table.Maintenances.MutableColumns.
			Except(table.Maintenances.CreatedAt),
		).
		MODEL(maint).
		RETURNING(table.Maintenances.AllColumns)

	err := stmt.QueryContext(ctx, s.db.Executor(ctx), maint)
	if err != nil {
		return nil, err
	}

	return fromDBMaintenance(maint), nil
}
