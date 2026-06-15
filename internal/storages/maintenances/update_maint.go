package maintenances

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func (s *Store) UpdateMaint(ctx context.Context, maint *entity.Maintenance) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Maintenances.UpdateMaint")
	defer span.End()

	// updated_at doubles as the optimistic-concurrency token (Maintenance.Revision)
	// compared on approve. The approve path runs under FOR UPDATE + SERIALIZABLE,
	// so two writes to the same row are serialized with real time between them —
	// the previous microsecond-collision concern on UnixMicro() is not reachable
	// there. Set it here so every update advances the token.
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
