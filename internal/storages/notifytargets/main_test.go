package notifytargets

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db         *sqlx.DB
	maintStore *maintenances.Store
)

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	maintStore = maintenances.NewStore(db)

	code := m.Run()
	os.Exit(code)
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

func makeNotifyTargets(ctx context.Context, t *testing.T, store *Store, maintID uuid.UUID) []*entity.NotifyTarget {
	t.Helper()

	notifyTargets, err := store.CreateMany(ctx, maintID, []*entity.NotifyTarget{
		{Transport: entity.NotifyTransportSlack, ChannelID: t.Name() + uuid.NewString()},
		{Transport: entity.NotifyTransportTelegram, ChannelID: t.Name() + uuid.NewString()},
	})
	require.NoError(t, err)

	return notifyTargets
}
