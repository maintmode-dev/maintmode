package audit

import (
	"context"
	"os"
	"testing"
	"time"

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
