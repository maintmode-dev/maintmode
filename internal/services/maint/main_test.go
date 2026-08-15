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
	users          *mock_maint.MockUserLister
	auditPublisher *mock_maint.MockAuditPublisher
	// audit records every published audit action so a test can assert which
	// action (and actor) a mutation produced. Always wired (publish is
	// fire-and-forget for most flows) — audit-focused tests read mocks.audit.
	audit *capturedAudit
}

func initService(t *testing.T) (*Service, serviceMocks) {
	t.Helper()

	ctrl := gomock.NewController(t)
	mocks := serviceMocks{
		users:          mock_maint.NewMockUserLister(ctrl),
		auditPublisher: mock_maint.NewMockAuditPublisher(ctrl),
		audit:          &capturedAudit{},
	}
	// Default: accept any publish, record it, succeed. One expectation, so the
	// recorder sees every call (a second AnyTimes stub would shadow this one).
	mocks.auditPublisher.EXPECT().
		Publish(gomock.Any(), gomock.Any()).
		DoAndReturn(mocks.audit.record).
		AnyTimes()

	txManager := dbtx.NewTxManager(db)
	notifyTargetsStore := notifytargetsstore.NewStore(db)

	taskStorage, err := goque.NewStorage(db)
	require.NoError(t, err)

	taskScheduler := scheduler.NewService(goque.NewTaskQueueManager(taskStorage))
	maintenancesStore := maintenances.NewStore(db)
	notifier, err := maintnotify.NewNotifier(
		cfg,
		messagesender.NewService(newStubResolver(), taskScheduler),
		notifyTargetsStore,
		stubOwnerResolver{},
		maintenancesStore,
		stubMentionResolver{},
	)
	require.NoError(t, err)

	return NewService(
		txManager,
		maintenancesStore,
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
		mocks.users,
		mocks.auditPublisher,
	), mocks
}

// newStubResolver builds the stub transport resolver: every Get short-circuits
// to the stub transport, so these service tests never touch a real transport or
// the DB-backed integration registry.
func newStubResolver() notifytransport.TransportResolver {
	return notifytransport.NewStubResolver()
}

// stubOwnerResolver names no one. Notifications are a side effect of the
// maintenance transitions under test here, not their subject, and a nil mention
// is the shape the renderer already handles (it simply omits the owner line), so
// there is nothing to assert and nothing to configure.
type stubOwnerResolver struct{}

func (stubOwnerResolver) ResolveOwner(_ context.Context, _ uuid.UUID) *entity.UserMention {
	return nil
}

// stubMentionResolver names no one either, for the same reason. The mentions
// themselves are read from the real store, so the delivery path stays wired.
type stubMentionResolver struct{}

func (stubMentionResolver) ResolveMentions(_ context.Context, _ []uuid.UUID) []*entity.UserMention {
	return nil
}

// expectAnyApproverEligible stubs the user service so every approver-eligibility
// check resolves eligible — the "approver validation is not what this test is
// about" default. It echoes back the queried ids as a non-empty result.
func (m serviceMocks) expectAnyApproverEligible() {
	m.users.EXPECT().
		ListUsers(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cmd *entity.ListUsersCmd) (*entity.ListUsersResult, error) {
			users := make([]*entity.User, 0, len(cmd.IDs))
			for _, id := range cmd.IDs {
				users = append(users, &entity.User{ID: id, Roles: entity.ApproverEligibleRoles})
			}
			return &entity.ListUsersResult{Users: users, Total: int64(len(users))}, nil
		}).
		AnyTimes()
}

// expectEligibleApprover stubs the user service so the given ids resolve as
// eligible approvers (a non-empty ListUsers result), and any other id resolves
// as ineligible (empty result). It mirrors the validateApprover composition:
// the service queries ListUsers by id with the eligible roles + ExcludeBlocked,
// and treats a non-empty result as eligible.
func (m serviceMocks) expectEligibleApprover(eligible ...uuid.UUID) {
	elig := make(map[uuid.UUID]struct{}, len(eligible))
	for _, id := range eligible {
		elig[id] = struct{}{}
	}
	m.users.EXPECT().
		ListUsers(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cmd *entity.ListUsersCmd) (*entity.ListUsersResult, error) {
			users := make([]*entity.User, 0, len(cmd.IDs))
			for _, id := range cmd.IDs {
				if _, ok := elig[id]; ok {
					users = append(users, &entity.User{ID: id, Roles: entity.ApproverEligibleRoles})
				}
			}
			return &entity.ListUsersResult{Users: users, Total: int64(len(users))}, nil
		}).
		AnyTimes()
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
	// CreateDraft rejects a window that starts in the past, so park the start in
	// the future — at a per-run-unique offset so parallel tests never overlap.
	start := xtime.UTCNow().Add(uniqueFutureOffset()).Round(time.Microsecond)
	notifyChannel := makeNotifyChannel(ctx, t, service)

	return &entity.CreateMaintenanceCmd{
		Title:           "Title" + t.Name(),
		Description:     "Description" + t.Name(),
		PlannedPeriod:   entity.NewPeriod(start, start.Add(time.Hour)),
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
