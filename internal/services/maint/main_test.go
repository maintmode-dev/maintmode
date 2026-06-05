package maint

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/ruko1202/goque"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/gateways/notifytransport"
	mock_maint "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/maint"
	"github.com/ruko1202/maintmode/internal/services/deferrednotifications"
	"github.com/ruko1202/maintmode/internal/services/maintnotify"
	"github.com/ruko1202/maintmode/internal/services/messaging/scheduler"
	messagesender "github.com/ruko1202/maintmode/internal/services/messaging/sender"
	deferrednotificationsstore "github.com/ruko1202/maintmode/internal/storages/deferrednotifications"
	notifytargetsstore "github.com/ruko1202/maintmode/internal/storages/notifytargets"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/conflicts"
	"github.com/ruko1202/maintmode/internal/services/notifytargets"
	conflictsnapshots "github.com/ruko1202/maintmode/internal/storages/conflict_snapshots"
	conflictsStore "github.com/ruko1202/maintmode/internal/storages/conflicts"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/notifychannel"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	testconfigutils "github.com/ruko1202/maintmode/test/utils/config"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db                 *sqlx.DB
	cfg                *config.AppConfig
	notifyChannelStore *notifychannel.Store
)

func TestMain(m *testing.M) {
	cfg = testconfigutils.LoadMaintConfig()
	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	notifyChannelStore = notifychannel.NewStore(db)

	code := m.Run()
	os.Exit(code)
}

type serviceMocks struct {
	approverValidator *mock_maint.MockApproverValidator
}

func initService(t *testing.T) (*Service, serviceMocks) {
	t.Helper()

	ctrl := gomock.NewController(t)
	mocks := serviceMocks{
		approverValidator: mock_maint.NewMockApproverValidator(ctrl),
	}

	txManager := dbtx.NewTxManager(db)
	notifyTargetsStore := notifytargetsstore.NewStore(db)

	taskStorage, err := goque.NewStorage(db)
	require.NoError(t, err)

	taskScheduler := scheduler.NewService(goque.NewTaskQueueManager(taskStorage))
	notifier, err := maintnotify.NewNotifier(
		cfg,
		messagesender.NewService(notifytransport.NewRegistry(cfg), taskScheduler),
		notifyTargetsStore,
	)
	require.NoError(t, err)

	return NewService(
		txManager,
		maintenances.NewStore(db),
		resources.NewStore(db),
		notifytargets.NewService(
			txManager,
			notifychannel.NewStore(db),
			notifyTargetsStore,
		),
		conflicts.NewService(
			conflictsStore.NewStore(db),
			conflictsnapshots.NewStore(db),
		),
		notifier,
		deferrednotifications.NewService(
			txManager,
			deferrednotificationsstore.NewStore(db),
			taskScheduler,
		),
		mocks.approverValidator,
	), mocks
}

func makeNotifyChannel(ctx context.Context, t *testing.T, service *Service) *entity.NotifyChannel {
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

// validCreateCmd builds a minimal create command that passes structural
// validation, so a test exercises only the approver-eligibility path.
func validCreateCmd(ctx context.Context, t *testing.T, service *Service, approverID uuid.UUID) *entity.CreateMaintenanceCmd {
	t.Helper()
	now := xtime.UTCNow().Round(time.Microsecond)
	notifyChannel := makeNotifyChannel(ctx, t, service)

	return &entity.CreateMaintenanceCmd{
		Title:           "Title" + t.Name(),
		Description:     "Description" + t.Name(),
		PlannedPeriod:   entity.NewPeriod(now, now.Add(time.Hour)),
		Impact:          entity.MaintenanceImpactFull,
		Scope:           entity.MaintenanceScopeResources,
		Resources:       []uuid.UUID{testdbutils.MakeResource(ctx, t, service.resourcesStore).ID},
		CreatedByUserID: uuid.New(),
		Steps: []*entity.MaintenanceStepInput{{
			Order:               1,
			Description:         "Step1" + t.Name(),
			RollbackDescription: "RollbackStep1" + t.Name(),
			DurationMinutes:     minStepDurationsMinutes,
		}},
		NotifyTargets:  []*entity.NotifyTargetInput{{ChannelID: notifyChannel.ID}},
		ApproverUserID: approverID,
	}
}
