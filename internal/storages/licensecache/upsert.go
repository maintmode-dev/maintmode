package licensecache

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// Upsert replaces the cached license with the given one. The table is a
// singleton (id locked to TRUE), so the write is INSERT .. ON CONFLICT DO
// UPDATE: the first successful heartbeat creates the row, every later one
// overwrites it in place.
func (s *Store) Upsert(ctx context.Context, license *entity.License) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.LicenseCache.Upsert")
	defer span.End()

	row := toDB(license)

	stmt := table.LicenseCache.
		INSERT(table.LicenseCache.AllColumns).
		MODEL(row).
		ON_CONFLICT(table.LicenseCache.ID).
		// seats_used is deliberately NOT updated here — it is the instance's own
		// counter, moved by the seat guards; a heartbeat only refreshes the
		// Console-owned fields.
		DO_UPDATE(postgres.SET(
			table.LicenseCache.Status.SET(table.LicenseCache.EXCLUDED.Status),
			table.LicenseCache.SeatsPurchased.SET(table.LicenseCache.EXCLUDED.SeatsPurchased),
			table.LicenseCache.FetchedAt.SET(table.LicenseCache.EXCLUDED.FetchedAt),
		))

	if _, err := stmt.ExecContext(ctx, s.db.Executor(ctx)); err != nil {
		xlog.Error(ctx, "upsert license cache failed", xfield.Error(err))
		return err
	}
	return nil
}
