package audit

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var db *sqlx.DB

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	code := m.Run()

	os.Exit(code)
}

// insertLogAt inserts one audit row with an explicit created_at so retention
// tests can backdate rows far into the past. (AddLog now also writes created_at
// from the entry, but this helper keeps the prune tests independent of the
// entity mapping.) The action carries a per-run unique marker so concurrent
// tests on the shared DB never count each other's rows.
func insertLogAt(ctx context.Context, t *testing.T, marker string, createdAt time.Time) {
	t.Helper()

	stmt := table.AuditLog.
		INSERT(table.AuditLog.Action, table.AuditLog.Actor, table.AuditLog.CreatedAt).
		MODEL(&model.AuditLog{
			Action:    marker,
			Actor:     "tester-" + xuuid.NewString(),
			CreatedAt: createdAt,
		})

	_, err := stmt.ExecContext(ctx, db)
	require.NoError(t, err)
}

// deleteAuditRowsOnCleanup removes, when the test ends, every audit row whose
// column equals value. The audit tests write to the shared dev database, and a
// row left behind is not inert: it shows up in the dev stand's audit screen
// and in anything captured from it (a UI wire-fixture capture once recorded
// `prune-expired-<uuid>` rows as if they were real actions). Each test scopes
// its rows by a unique marker or actor, so that value is also the exact
// cleanup key.
func deleteAuditRowsOnCleanup(t *testing.T, column postgres.ColumnString, value string) {
	t.Helper()

	t.Cleanup(func() {
		// Not t.Context(): it is already canceled when cleanups run.
		_, err := table.AuditLog.DELETE().
			WHERE(column.EQ(postgres.String(value))).
			ExecContext(context.Background(), db)
		require.NoError(t, err, "clean up audit rows where %s = %q", column.Name(), value)
	})
}

// newMarker returns a per-run unique action marker for the retention tests and
// deletes every row carrying it when the test ends.
func newMarker(t *testing.T, prefix string) string {
	t.Helper()

	marker := prefix + xuuid.NewString()
	deleteAuditRowsOnCleanup(t, table.AuditLog.Action, marker)

	return marker
}
