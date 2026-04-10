package maintenances

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"

	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/utils/closer"
)

var db *sqlx.DB

func TestMain(m *testing.M) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	xlog.ReplaceGlobalLogger(xlog.NewZapAdapter(logger))
	conn := testdbconnutils.NewDB()
	db = conn
	closer.Add(conn.Close)

	code := m.Run()

	closer.CloseAll(ctx)

	os.Exit(code)
}

func makeMaint(ctx context.Context, t *testing.T, store *Store, period entity.Period) *entity.Maintenance {
	t.Helper()

	maint := &entity.Maintenance{
		Title:         "Title" + t.Name(),
		Description:   "Description" + t.Name(),
		PlannedPeriod: period,
		Scope:         entity.MaintenanceScopeResources,
		Status:        entity.MaintenanceStatusPlanned,
		Impact:        entity.MaintenanceImpactFull,
		Resources: []*entity.Resource{
			{
				ID:   xuuid.New(),
				Type: entity.ResourceTypeService,
			}, {
				ID:   xuuid.New(),
				Type: entity.ResourceTypeDatabase,
			},
		},
	}

	created, err := store.Create(ctx, maint)
	require.NoError(t, err)

	err = store.AddResources(ctx, created.ID, maint.Resources)
	require.NoError(t, err)
	created.Resources = maint.Resources

	return created
}
