package deferrednotifications

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	mock_deferrednotifications "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/deferrednotifications"
	deferredstore "github.com/ruko1202/maintmode/internal/storages/deferrednotifications"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db         *sqlx.DB
	maintStore *maintenances.Store
)

func TestMain(m *testing.M) {
	cfg := config.LoadAppConfig()
	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	maintStore = maintenances.NewStore(db)

	code := m.Run()
	os.Exit(code)
}

type serviceMocks struct {
	scheduler *mock_deferrednotifications.MockScheduler
}

func initService(t *testing.T) (*Service, *serviceMocks) {
	t.Helper()

	ctrl := gomock.NewController(t)
	mocks := &serviceMocks{
		scheduler: mock_deferrednotifications.NewMockScheduler(ctrl),
	}

	return NewService(
		dbtx.NewTxManager(db),
		deferredstore.NewStore(db),
		mocks.scheduler,
	), mocks
}

func makeMaint(ctx context.Context, t *testing.T) *entity.Maintenance {
	t.Helper()
	start := xtime.UTCNow()
	maint, err := maintStore.CreateMaint(ctx, &entity.Maintenance{
		Title:         "test-" + t.Name(),
		Description:   "test",
		PlannedPeriod: entity.NewPeriod(start, start.Add(time.Hour)),
		Scope:         entity.MaintenanceScopeGlobal,
		Status:        entity.MaintenanceStatusDraft,
		Impact:        entity.MaintenanceImpactNone,
	})
	require.NoError(t, err)
	return maint
}

func makeNotifications(ctx context.Context, t *testing.T, svc *Service) (*entity.Maintenance, []*entity.DeferredNotification) {
	t.Helper()

	maint := makeMaint(ctx, t)
	fireAt := xtime.UTCNow().Add(30 * time.Minute)

	// Create persists the schedule; nothing enqueued yet.
	created, err := svc.Create(ctx, maint.ID, []*entity.DeferredNotification{
		{FireAt: fireAt},
		{FireAt: fireAt.Add(25 * time.Minute)},
	})
	require.NoError(t, err)
	require.Len(t, created, 2)

	return maint, created
}
