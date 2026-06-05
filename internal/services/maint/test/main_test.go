package test

import (
	"os"
	"testing"

	"github.com/jmoiron/sqlx"

	testbootstraputils "github.com/ruko1202/maintmode/test/utils/bootstrap"
	testconfigutils "github.com/ruko1202/maintmode/test/utils/config"

	"github.com/ruko1202/maintmode/internal/config"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"

	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/closer"
)

var (
	db             *sqlx.DB
	cfg            *config.AppConfig
	maintStore     *maintenances.Store
	resourcesStore *resources.Store
)

func TestMain(m *testing.M) {
	cfg = testconfigutils.LoadMaintConfig()

	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	stores := testbootstraputils.InitStores(db)
	resourcesStore = stores.Resources
	maintStore = stores.Maintenances

	code := m.Run()

	os.Exit(code)
}
