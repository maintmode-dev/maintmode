package audit

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	auditaction "github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/services/auditor"
	"github.com/ruko1202/maintmode/internal/storages/audit"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db *sqlx.DB
)

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAuthAppConfig())
	closer.Add(db.Close)

	code := m.Run()

	os.Exit(code)
}

// seedAuditFunc writes the audit row an action would produce, synchronously —
// it runs the same render + AddLog path the audit-write goque processor runs in
// production (RUK-179), without spinning up a worker. Tests use it to seed rows
// and then read them back through the API.
type seedAuditFunc func(ctx context.Context, action auditaction.Action)

func initImpl(t *testing.T) (*Implementation, seedAuditFunc) {
	t.Helper()

	// Read side (Implementation) and the seed helper share one Auditor: the test
	// seeds records via the render+write path and reads them through the API.
	auditorSrv := auditor.NewAuditor(audit.NewStore(db))
	renderer := auditaction.NewRenderer()

	seed := func(ctx context.Context, action auditaction.Action) {
		payload, err := renderer.Render(action)
		require.NoError(t, err)
		require.NoError(t, auditorSrv.AddLog(ctx, payload.ToAuditEntry()))
	}

	return New(auditorSrv), seed
}
