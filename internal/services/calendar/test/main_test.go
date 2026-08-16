package test

import (
	"os"
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/ruko1202/maintmode/internal/config"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"

	"github.com/ruko1202/maintmode/internal/services/calendar"
	"github.com/ruko1202/maintmode/internal/services/conflicts"
	conflictsnapshots "github.com/ruko1202/maintmode/internal/storages/conflict_snapshots"
	conflictsstore "github.com/ruko1202/maintmode/internal/storages/conflicts"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/closer"
)

var (
	db             *sqlx.DB
	maintStore     *maintenances.Store
	resourcesStore *resources.Store
	snapshotStore  *conflictsnapshots.Store
)

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	maintStore = maintenances.NewStore(db)
	resourcesStore = resources.NewStore(db)
	snapshotStore = conflictsnapshots.NewStore(db)

	code := m.Run()

	os.Exit(code)
}

// initService builds the calendar service against the real database. The
// branch under test picks between three conflict sources, and two of them are
// SQL — mocking the stores would leave the interesting half unexercised.
//
// Notify targets and deferred notifications are nil: nothing on the conflict
// read path touches them.
func initService(db *sqlx.DB) *calendar.Service {
	return calendar.NewService(
		maintenances.NewStore(db),
		resources.NewStore(db),
		nil,
		nil,
		conflicts.NewService(
			conflictsstore.NewStore(db),
			conflictsnapshots.NewStore(db),
		),
	)
}
