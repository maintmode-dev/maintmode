package audit

import (
	"os"
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/eventbus"
	auditorlistener "github.com/ruko1202/maintmode/internal/eventbus/listeners/auditor"
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

func initImpl(t *testing.T) (*Implementation, *eventbus.Dispatcher) {
	t.Helper()

	// Read-сторона (Implementation) и write-сторона (Dispatcher с аудит-листенером)
	// делят один Auditor: тест сеет записи через Dispatch и читает их через API.
	auditorSrv := auditor.NewAuditor(audit.NewStore(db))
	return New(auditorSrv), eventbus.NewDispatcher(auditorlistener.NewListener(auditorSrv))
}
