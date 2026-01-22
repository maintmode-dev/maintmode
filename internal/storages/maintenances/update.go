package maintenances

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/postgres/public/table"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func (s *Store) Update(ctx context.Context, maint *entity.Maintenance) error {
	ctx = xlog.WithOperation(ctx, "store.Maintenances.Update")

	maint.UpdatedAt = lo.ToPtr(xtime.UTCNow())

	stmt := table.Maintenances.
		UPDATE(table.Maintenances.MutableColumns).
		MODEL(toDBMaintenance(maint)).
		WHERE(table.Maintenances.ID.EQ(postgres.UUID(maint.ID)))

	_, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return err
	}

	return nil
}
