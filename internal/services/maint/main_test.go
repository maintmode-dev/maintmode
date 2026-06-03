package maint

import (
	"context"
	"os"
	"testing"

	"github.com/ruko1202/goque"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/notifytargets"
	"github.com/ruko1202/maintmode/internal/storages/notifychannel"
	notifytargetsstore "github.com/ruko1202/maintmode/internal/storages/notifytargets"

	"github.com/ruko1202/maintmode/internal/gateways/notifytransport"
	"github.com/ruko1202/maintmode/internal/services/conflicts"
	"github.com/ruko1202/maintmode/internal/services/deferrednotifications"
	"github.com/ruko1202/maintmode/internal/services/maintnotify"
	"github.com/ruko1202/maintmode/internal/services/messaging/scheduler"
	messagesender "github.com/ruko1202/maintmode/internal/services/messaging/sender"
	conflictsnapshots "github.com/ruko1202/maintmode/internal/storages/conflict_snapshots"
	conflictsStore "github.com/ruko1202/maintmode/internal/storages/conflicts"
	deferrednotificationsstore "github.com/ruko1202/maintmode/internal/storages/deferrednotifications"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"

	testconfigutils "github.com/ruko1202/maintmode/test/utils/config"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	resourcesStore     *resources.Store
	notifyChannelStore *notifychannel.Store
	service            *Service
)

func TestMain(m *testing.M) {
	cfg := testconfigutils.LoadMaintConfig()
	db := testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	resourcesStore = resources.NewStore(db)
	notifyChannelStore = notifychannel.NewStore(db)
	txManager := dbtx.NewTxManager(db)
	notifyTargetsStore := notifytargetsstore.NewStore(db)

	taskStorage, err := goque.NewStorage(db)
	if err != nil {
		panic(err)
	}
	taskScheduler := scheduler.NewService(goque.NewTaskQueueManager(taskStorage))
	sender := messagesender.NewService(notifytransport.NewRegistry(cfg), taskScheduler)
	notifier, err := maintnotify.NewNotifier(cfg, sender, notifyTargetsStore)
	if err != nil {
		panic(err)
	}

	deferred := deferrednotifications.NewService(
		txManager,
		deferrednotificationsstore.NewStore(db),
		taskScheduler,
	)

	service = NewService(
		txManager,
		maintenances.NewStore(db),
		resourcesStore,
		notifytargets.NewService(txManager, notifyChannelStore, notifyTargetsStore),
		conflicts.NewService(
			conflictsStore.NewStore(db),
			conflictsnapshots.NewStore(db),
		),
		notifier,
		deferred,
	)

	code := m.Run()
	os.Exit(code)
}

func makeNotifyChannel(ctx context.Context, t *testing.T) *entity.NotifyChannel {
	t.Helper()

	// Catalog now lives in Postgres: seed a channel for this test, then
	// assert it surfaces through the service-facing AvailableChannels.
	channel, err := notifyChannelStore.Create(ctx, &entity.NotifyChannel{
		Transport:          entity.NotifyTransportTelegram,
		TransportChannelID: t.Name() + xuuid.NewString(),
		Name:               t.Name(),
		Description:        t.Name(),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)

	notifyChannels, err := service.notifyTargets.AvailableChannels(ctx, false)
	require.NoError(t, err)
	require.NotEmpty(t, notifyChannels)

	return channel
}
