package claimer

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/ruko1202/goque"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	mock_user "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/user"
	"github.com/ruko1202/maintmode/internal/services/auditpublisher"
	"github.com/ruko1202/maintmode/internal/services/user"
	"github.com/ruko1202/maintmode/internal/storages/useridentities"
	"github.com/ruko1202/maintmode/internal/storages/userinvitations"
	"github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db  *sqlx.DB
	cfg *config.AppConfig

	// loginProviders resolves the provider names these tests sign in with. See
	// testdbutils.SeedLoginProviders for why the rows have to exist at all.
	loginProviders *testdbutils.LoginProviders
)

func TestMain(m *testing.M) {
	cfg = config.LoadAppConfig()
	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	loginProviders = testdbutils.MustSeedLoginProviders(context.Background(), db,
		"claimer-suite-kek", entity.AuthMethodGoogle)

	code := m.Run()
	os.Exit(code)
}

// harness is a claimer over the real invitation store and user service, with
// the two collaborators a test drives replaced by fakes.
type harness struct {
	claimer *Claimer
	store   *userinvitations.Store
	users   *user.Service
	// handles is the dance handle store the claimer is built with. Tests park
	// handles in it, or make it fail, before calling the claimer.
	handles *fakeDanceHandles
	// seatGuard runs inside AssignRoles, on the claim's own transaction.
	seatGuard *fakeSeatGuard
}

func initHarness(t *testing.T) *harness {
	t.Helper()

	txManager := dbtx.NewTxManager(db)
	h := &harness{
		store:     userinvitations.NewStore(db),
		handles:   newFakeDanceHandles(),
		seatGuard: &fakeSeatGuard{},
	}
	h.users = user.NewService(
		txManager,
		users.NewStore(db),
		useridentities.NewStore(db),
		newTestAuditPublisher(t),
		mock_user.NewMockTokenRevoker(gomock.NewController(t)),
		h.seatGuard,
		false,
		loginProviders,
	)
	h.claimer = New(txManager, h.store, h.users, h.handles)

	return h
}

// fakeSeatGuard counts its calls and lets a test observe the world as the real
// guard would: inside the claim transaction, where the real guard's seat count
// reads the invitations table.
type fakeSeatGuard struct {
	called int
	onCall func(ctx context.Context)
}

func (f *fakeSeatGuard) EnsureSeatAvailable(ctx context.Context) error {
	f.called++
	if f.onCall != nil {
		f.onCall(ctx)
	}

	return nil
}

func newTestAuditPublisher(t *testing.T) *auditpublisher.Publisher {
	t.Helper()
	storage, err := goque.NewStorage(db)
	require.NoError(t, err)

	return auditpublisher.New(goque.NewTaskQueueManager(storage))
}

// uniqueEmail returns a per-run-unique address, so `-count 2` on the shared
// database never collides.
func uniqueEmail(t *testing.T) string {
	t.Helper()

	return xuuid.NewString() + "@claimer-test.com"
}

// makeUser creates a real user: an invitation's inviter, or its claimant.
func makeUser(ctx context.Context, t *testing.T, h *harness) *entity.User {
	t.Helper()

	u, err := h.users.GetOrCreateByAuthInfo(ctx, entity.AuthMethodGoogle, &entity.OAuthProviderUserInfo{
		ID:    xuuid.NewString(),
		Email: uniqueEmail(t),
		Name:  "Claimer test user",
	}, entity.UserCreationPolicy{AllowCreate: true})
	require.NoError(t, err)

	return u
}

// createInvitation stores an invitation for email straight through the store:
// the claimer spends invitations, it does not issue them.
func createInvitation(
	ctx context.Context,
	t *testing.T,
	h *harness,
	email string,
	expiresAt time.Time,
	roles ...entity.Role,
) *entity.Invitation {
	t.Helper()

	if len(roles) == 0 {
		roles = []entity.Role{entity.RoleEditor}
	}

	inv, err := h.store.Create(ctx, &entity.Invitation{
		Email:       email,
		Roles:       roles,
		TokenHash:   xhash.HashSha256([]byte(xuuid.NewString())),
		Status:      entity.InvitationStatusPending,
		ExpiresAt:   expiresAt,
		SentAt:      xtime.UTCNow().Add(-time.Minute),
		InvitedByID: makeUser(ctx, t, h).ID,
	})
	require.NoError(t, err)

	return inv
}

// createPendingInvitation stores a live invitation for email.
func createPendingInvitation(ctx context.Context, t *testing.T, h *harness, email string, roles ...entity.Role) *entity.Invitation {
	t.Helper()

	return createInvitation(ctx, t, h, email, xtime.UTCNow().Add(24*time.Hour), roles...)
}
