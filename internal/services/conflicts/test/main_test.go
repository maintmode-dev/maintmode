package test

import (
	"os"
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/services/conflicts"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"

	conflictsnapshots "github.com/ruko1202/maintmode/internal/storages/conflict_snapshots"
	conflictsStore "github.com/ruko1202/maintmode/internal/storages/conflicts"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/closer"
)

var (
	db             *sqlx.DB
	maintStore     *maintenances.Store
	resourcesStore *resources.Store
)

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	maintStore = maintenances.NewStore(db)
	resourcesStore = resources.NewStore(db)
	code := m.Run()

	os.Exit(code)
}

func initService(db *sqlx.DB) *conflicts.Service {
	return conflicts.NewService(
		conflictsStore.NewStore(db),
		conflictsnapshots.NewStore(db),
	)
}
