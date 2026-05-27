package maint

import (
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/gateways/messengers"
	"github.com/ruko1202/maintmode/internal/services/conflicts"
	"github.com/ruko1202/maintmode/internal/services/maintnotify"
	"github.com/ruko1202/maintmode/internal/services/messaging/sender"
	conflictsnapshots "github.com/ruko1202/maintmode/internal/storages/conflict_snapshots"
	conflictsStore "github.com/ruko1202/maintmode/internal/storages/conflicts"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"

	testconfigutils "github.com/ruko1202/maintmode/test/utils/config"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

// testServices is the maint-package equivalent of bootstrap.Services.
// We can't reuse bootstrap.Services here because that would create an
// import cycle (bootstrap → maint, and maint tests would import bootstrap).
type testServices struct {
	Maint *Service
}

var (
	db             *sqlx.DB
	resourcesStore *resources.Store
	services       *testServices
)

func TestMain(m *testing.M) {
	cfg := testconfigutils.LoadMaintConfig()
	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	resourcesStore = resources.NewStore(db)

	// Wire the full notifier stack so lifecycle tests that emit
	// notifications (e.g. step.cancelled) don't nil-panic. The
	// MessengerRegistry routes everything to the stub messenger when
	// cfg.Environment.IsDev() && cfg.Messengers.UseStub — both are true
	// in the local test config.
	taskStorage, err := goque.NewStorage(db)
	if err != nil {
		panic(err)
	}
	queue := goque.NewTaskQueueManager(taskStorage)
	messengerRegistry := messengers.NewMessengerRegistry(cfg)
	senderSvc := sender.NewMessengerService(messengerRegistry, queue)

	notifier, err := maintnotify.NewNotifier(cfg, senderSvc)
	if err != nil {
		panic(err)
	}

	services = &testServices{
		Maint: NewService(
			dbtx.NewTxManager(db),
			maintenances.NewStore(db),
			resourcesStore,
			conflicts.NewService(
				conflictsStore.NewStore(db),
				conflictsnapshots.NewStore(db),
			),
			notifier,
		),
	}

	code := m.Run()
	os.Exit(code)
}
