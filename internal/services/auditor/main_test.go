package auditor

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
	"github.com/ruko1202/maintmode/internal/storages/audit"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db    *sqlx.DB
	store *audit.Store
)

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	store = audit.NewStore(db)

	code := m.Run()
	os.Exit(code)
}

// insertLogAt writes one audit row at an explicit created_at under a per-run
// marker, so the prune drain-loop can be exercised on backdated rows scoped to
// the test. AddLog always stamps NOW(), so the test bypasses it.
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

func countByMarker(ctx context.Context, t *testing.T, marker string) int64 {
	t.Helper()

	var dest struct {
		Count int64 `alias:"c"`
	}
	stmt := table.AuditLog.
		SELECT(postgres.COUNT(postgres.STAR).AS("c")).
		WHERE(table.AuditLog.Action.EQ(postgres.String(marker)))
	require.NoError(t, stmt.QueryContext(ctx, db, &dest))

	return dest.Count
}
