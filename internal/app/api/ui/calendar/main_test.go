package uicalendar

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5/echotest"
	redisDB "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	apinotifications "github.com/ruko1202/maintmode/internal/app/api/public/notifytargets"
	notificationsmodels "github.com/ruko1202/maintmode/internal/app/api/public/notifytargets/models"
	"github.com/ruko1202/maintmode/internal/utils/xtime"

	testbootstraputils "github.com/ruko1202/maintmode/test/utils/bootstrap"
	testconfigutils "github.com/ruko1202/maintmode/test/utils/config"

	maintapi "github.com/ruko1202/maintmode/internal/app/api/public/maint"
	maintmodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	"github.com/ruko1202/maintmode/internal/app/bootstrap"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

var (
	db    *sqlx.DB
	redis *redisDB.Client
	cfg   *config.AppConfig
)

func TestMain(m *testing.M) {
	cfg = testconfigutils.LoadMaintConfig()

	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	redis = testdbconnutils.NewRedisClient(cfg)
	closer.Add(redis.Close)

	code := m.Run()

	os.Exit(code)
}

const maintmodePolicy = `
g, editor, guest
g, reviewer, editor
g, admin, reviewer

p, guest, calendar.read, execute
p, guest, maintenance.read, execute
p, guest, resource.read, execute

p, editor, maintenance.create, execute
p, editor, maintenance.edit, execute
p, editor, maintenance.start, execute
p, editor, maintenance.complete, execute
p, editor, maintenance.cancel, execute
p, editor, maintenance.step.start, execute
p, editor, maintenance.step.complete, execute
p, editor, maintenance.step.cancel, execute
p, editor, resource.create, execute

p, reviewer, maintenance.approve, execute
`

// newServices builds a per-test service set wired to the real auth services
// (user/auth in-process) backed by the live Postgres and Redis.
func newServices(ctx context.Context, t *testing.T) *bootstrap.Services {
	t.Helper()

	return testbootstraputils.InitServicesT(ctx, t, db, redis, cfg)
}

func initImpl(t *testing.T) *Implementation {
	t.Helper()

	services := newServices(context.Background(), t)
	return New(services.Calendar, services.RBAC, services.UserSummary)
}

func makeMaint(ctx context.Context, t *testing.T) *maintmodels.CreateDraftMaintResponse {
	t.Helper()

	services := newServices(ctx, t)
	maintImpl := maintapi.New(services.Maint, services.UserSummary)

	// A real, persisted, approver-eligible user: CreateDraftMaint validates the
	// approver against the real user backend.
	approver := testbootstraputils.SeedEligibleApprover(ctx, t, services)

	notifyChan := makeNotifyChannel(ctx, t)
	req := &maintmodels.CreateDraftMaintRequest{
		Title:        "Test maint for calendar view " + uuid.New().String()[:8],
		Description:  "Test description",
		PlannedStart: xtime.UTCNow(),
		Scope:        maintmodels.MaintenanceScopeGlobal,
		Impact:       maintmodels.MaintenanceImpactNone,
		Steps: []*maintmodels.MaintenanceStepInput{{
			Order:               1,
			Description:         "Step 1",
			RollbackDescription: "Rollback 1",
			Duration:            "30m",
		}},
		NotifyTargets: &maintmodels.NotifyTargets{
			ChannelIDs: []string{notifyChan.ID},
		},
		ApproverUserID: approver.ID,
	}

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, req),
	}.ToContextRecorder(t)
	c.SetRequest(c.Request().WithContext(ctx))
	// CreateDraftMaint captures the author from the Echo context (set by the
	// auth middleware in production), so the setup helper must seed a user.
	xecho.UserToEchoCtx(c, &entity.User{
		ID:    uuid.New(),
		Email: "author-" + uuid.NewString() + "@example.com",
		Name:  "Author " + t.Name(),
		Roles: entity.DefaultRoles,
	})

	err := maintImpl.CreateDraftMaint(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	resp := testjsonudils.JSONToAny[maintmodels.CreateDraftMaintResponse](t, rec.Body)
	return &resp
}

// makeNotifyChannel seeds one catalog channel via the admin create endpoint
// and returns it. The catalog now lives in Postgres, so tests must create the
// channel they reference rather than relying on config-seeded stubs.
func makeNotifyChannel(ctx context.Context, t *testing.T) *notificationsmodels.Channel {
	t.Helper()

	services := newServices(ctx, t)
	impl := apinotifications.New(services.NotifyTargets, services.UserSummary, services.TransportResolver)

	req := &notificationsmodels.CreateChannelRequest{
		Transport:          string(entity.NotifyTransportTelegram),
		TransportChannelID: t.Name() + "-" + uuid.NewString(),
		Name:               t.Name(),
		Description:        "test channel",
	}

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, req),
	}.ToContextRecorder(t)
	c.SetRequest(c.Request().WithContext(ctx))
	xecho.UserToEchoCtx(c, &entity.User{ID: uuid.New(), Name: t.Name(), Email: t.Name() + "@example.com"})

	err := impl.CreateChannel(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, rec.Code)

	return testjsonudils.JSONToAny[*notificationsmodels.Channel](t, rec.Body)
}
