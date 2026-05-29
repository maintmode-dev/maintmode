package uicalendar

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5/echotest"
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
	"github.com/ruko1202/maintmode/internal/utils/closer"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

var (
	db        *sqlx.DB
	cfg       *config.AppConfig
	services  *bootstrap.Services
	maintImpl *maintapi.Implementation
)

func TestMain(m *testing.M) {
	cfg = testconfigutils.LoadMaintConfig()

	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	services = testbootstraputils.InitServices(context.Background(), db, cfg)

	maintImpl = maintapi.New(services.Maint)

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

func initImpl(t *testing.T) *Implementation {
	t.Helper()

	return New(services.Calendar, services.RBAC)
}

func makeMaint(ctx context.Context, t *testing.T) *maintmodels.CreateDraftMaintResponse {
	t.Helper()

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
	}

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, req),
	}.ToContextRecorder(t)
	c.SetRequest(c.Request().WithContext(ctx))

	err := maintImpl.CreateDraftMaint(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	resp := testjsonudils.JSONToAny[maintmodels.CreateDraftMaintResponse](t, rec.Body)
	return &resp
}

func makeNotifyChannel(ctx context.Context, t *testing.T) *notificationsmodels.Channel {
	t.Helper()

	impl := apinotifications.New(services.NotifyTargets)

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	c.SetRequest(c.Request().WithContext(ctx))

	err := impl.GetChannels(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	channels := testjsonudils.JSONToAny[notificationsmodels.ChannelsResponse](t, rec.Body)

	return channels.Channels[0]
}
