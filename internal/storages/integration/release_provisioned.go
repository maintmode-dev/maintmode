package integration

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// ReleaseProvisioned hands the named rows of a kind back to the admin API: not
// provisioned, disabled, no editor. Config and secrets are left as they are --
// a released row has no secret, and an admin supplies one to enable it again.
func (s *Store) ReleaseProvisioned(ctx context.Context, kind string, names []string) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Integration.ReleaseProvisioned")
	defer span.End()

	if len(names) == 0 {
		return nil
	}

	stmt := table.IntegrationSettings.
		UPDATE().
		SET(
			table.IntegrationSettings.Provisioned.SET(postgres.Bool(false)),
			table.IntegrationSettings.Enabled.SET(postgres.Bool(false)),
			table.IntegrationSettings.UpdatedByUserID.SET(postgres.StringExp(postgres.NULL)),
			table.IntegrationSettings.UpdatedAt.SET(postgres.TimestampzT(xtime.UTCNow())),
		).
		WHERE(table.IntegrationSettings.Kind.EQ(postgres.String(kind)).
			AND(table.IntegrationSettings.Name.IN(lo.Map(names, func(name string, _ int) postgres.Expression {
				return postgres.String(name)
			})...)))

	_, err := stmt.ExecContext(ctx, s.db.Executor(ctx))

	return err
}
