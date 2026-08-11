package uiapprovals

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	valkeyDB "github.com/redis/go-redis/v9"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	testbootstraputils "github.com/ruko1202/maintmode/test/utils/bootstrap"
	testconfigutils "github.com/ruko1202/maintmode/test/utils/config"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db     *sqlx.DB
	valkey *valkeyDB.Client
	cfg    *config.AppConfig
)

func TestMain(m *testing.M) {
	cfg = testconfigutils.LoadMaintConfig()

	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	valkey = testdbconnutils.NewValkeyClient(cfg)
	closer.Add(valkey.Close)

	code := m.Run()

	os.Exit(code)
}

func initImpl(t *testing.T) *Implementation {
	t.Helper()

	services := testbootstraputils.InitServicesT(context.Background(), t, db, valkey, cfg)

	return New(services.Calendar, services.UserSummary)
}

// makePendingMaint seeds a draft maintenance awaiting the given approver.
//
// Written straight to the stores rather than through CreateDraftMaint on
// purpose: that path validates the approver against the real user backend and
// would drag in SeedEligibleApprover, whose contention over the last active
// admin's roles flakes on the shared DB. Nothing in this handler cares whether
// the approver is a real user — the id is a filter value.
func makePendingMaint(
	ctx context.Context,
	t *testing.T,
	approverID uuid.UUID,
	changers ...testdbutils.MaintChanger,
) *entity.Maintenance {
	t.Helper()

	start := xtime.UTCNow().Add(24 * time.Hour)

	return testdbutils.MakeMaint(
		ctx, t,
		maintenances.NewStore(db),
		resources.NewStore(db),
		entity.NewPeriod(start, start.Add(2*time.Hour)),
		append([]testdbutils.MaintChanger{
			testdbutils.WithApprover(approverID),
			testdbutils.WithStatus(entity.MaintenanceStatusDraft),
		}, changers...)...,
	)
}
