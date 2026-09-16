package integration

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// Delete removes one integration by its identity. Not-found is
// ErrIntegrationNotFound rather than a silent no-op: a delete that reports
// success for a row that was never there hides a typo in the name.
//
// The secrets go with the row, and they are unrecoverable afterwards -- which
// is why the caller checks for linked accounts BEFORE calling this.
//
// The row's data_keys entry is left behind. Create mints a fresh DEK per
// integration, so each delete orphans exactly one, which at admin rates is
// nothing worth a cascade that could reach a key another row still depends on.
func (s *Store) Delete(ctx context.Context, kind, name string) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Integration.Delete")
	defer span.End()

	stmt := table.IntegrationSettings.
		DELETE().
		WHERE(table.IntegrationSettings.Kind.EQ(postgres.String(kind)).
			AND(table.IntegrationSettings.Name.EQ(postgres.String(name)))).
		RETURNING(table.IntegrationSettings.ID)

	var deleted []model.IntegrationSettings
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &deleted); err != nil {
		return err
	}
	if len(deleted) == 0 {
		return apperr.ErrIntegrationNotFound
	}

	return nil
}
