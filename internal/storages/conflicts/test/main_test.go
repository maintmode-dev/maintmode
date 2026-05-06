package test

import (
	"os"
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/ruko1202/maintmode/internal/config"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"

	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/closer"
)

var (
	db             *sqlx.DB
	maintsStore    *maintenances.Store
	resourcesStore *resources.Store
)

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	maintsStore = maintenances.NewStore(db)
	resourcesStore = resources.NewStore(db)

	code := m.Run()

	os.Exit(code)
}
