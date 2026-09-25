package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	valkeyDB "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/services/authmethod"
	authflags "github.com/ruko1202/maintmode/test/utils/mocks/authflags"

	"github.com/ruko1202/maintmode/internal/config"

	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/services/auditpublisher"

	mock_auth "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/auth"
	mock_authmethod "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/authmethod"

	"github.com/ruko1202/maintmode/internal/services/user"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/authmethod/bootstrapauth"
	"github.com/ruko1202/maintmode/internal/services/license"
	"github.com/ruko1202/maintmode/internal/services/token"
	"github.com/ruko1202/maintmode/internal/storages/authcredentials"
	"github.com/ruko1202/maintmode/internal/storages/blacklisttoken"
	"github.com/ruko1202/maintmode/internal/storages/distributedlock"
	"github.com/ruko1202/maintmode/internal/storages/refreshtoken"
	"github.com/ruko1202/maintmode/internal/storages/useridentities"
	"github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db     *sqlx.DB
	valkey *valkeyDB.Client
	cfg    *config.AppConfig
)

const (
	tokenIssuer = "test-issuer"
)

func TestMain(m *testing.M) {
	cfg = config.LoadAppConfig()
	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	valkey = testdbconnutils.NewValkeyClient(cfg)
	closer.Add(valkey.Close)

	loginProviders = testdbutils.MustSeedLoginProviders(context.Background(), db,
		"auth-suite-kek", entity.AuthMethodGoogle, entity.AuthMethodGithub)

	code := m.Run()

	os.Exit(code)
}

type serviceMocks struct {
	authMethod   *mock_authmethod.MockAuthMethod
	otpVerifier  *mock_auth.MockOTPVerifier
	otpRequester *mock_auth.MockOTPRequester
}

func initService(t *testing.T) (*Service, *serviceMocks) {
	t.Helper()
	return initServiceForMethod(t, entity.AuthMethodGoogle)
}

// initInviteOnlyService builds the service with open signup DISABLED, which is
// the production shape and the only one where AllowCreate decides anything.
// The shared local config runs open signup, so a test built on initService
// cannot tell a correct AllowCreate gate from one hard-coded true.
func initInviteOnlyService(t *testing.T) (*Service, *serviceMocks) {
	t.Helper()
	return initServiceWith(t, entity.AuthMethodGoogle, false)
}

// initServiceForMethod builds the service with its single mock auth method
// registered under methodID. The registry keys providers by MethodID(), so a
// test exercising the password login must register the mock as
// AuthMethodBootstrap or Get would not find it.
func initServiceForMethod(t *testing.T, methodID entity.AuthMethod) (*Service, *serviceMocks) {
	t.Helper()
	return initServiceWith(t, methodID, true)
}

func initServiceWith(t *testing.T, methodID entity.AuthMethod, allowOpenSignup bool) (*Service, *serviceMocks) {
	t.Helper()

	return initServiceWithDeps(t, methodID, serviceDeps{inviteOnly: !allowOpenSignup})
}

// initServiceWithBootstrap builds the service around a REAL break-glass
// provider answering for the given address and password.
//
// The mock AuthMethod cannot stand in here: the seed path needs the provider to
// report which address it serves and whether its password is durable, and a
// bare AuthMethod says neither. Substituting a mock would make every seed test
// silently take the "not the bootstrap address" branch.
func initServiceWithBootstrap(t *testing.T, email, password string) (*Service, *serviceMocks) {
	t.Helper()

	return initServiceWithBootstrapFlags(t, email, password, nil)
}

// initServiceWithBootstrapFlags is initServiceWithBootstrap with the built-in
// method flags the sign-in gates read. Nil means every built-in is offered.
func initServiceWithBootstrapFlags(
	t *testing.T, email, password string, flags AuthMethodFlags,
) (*Service, *serviceMocks) {
	t.Helper()

	return initServiceWithDeps(t, entity.AuthMethodBootstrap, serviceDeps{
		concrete: bootstrapauth.NewService(config.BootstrapConfig{Email: email}, password),
		flags:    flags,
	})
}

// initServiceWithUnrelatedBootstrap builds the service for a test that exercises
// something other than the break-glass path -- an ordinary password login, a
// change, a reset. A real bootstrap provider still has to be registered, since
// the fallback branch consults it on every login, so it answers for an address
// and a password no test in this package submits.
func initServiceWithUnrelatedBootstrap(t *testing.T) (*Service, *serviceMocks) {
	t.Helper()

	return initServiceWithUnrelatedBootstrapFlags(t, nil)
}

// initServiceWithUnrelatedBootstrapFlags is initServiceWithUnrelatedBootstrap
// with the built-in method flags the sign-in gates read.
func initServiceWithUnrelatedBootstrapFlags(t *testing.T, flags AuthMethodFlags) (*Service, *serviceMocks) {
	t.Helper()

	return initServiceWithBootstrapFlags(t, bootstrapAddress(), "unrelated-"+xuuid.NewString(), flags)
}

// serviceDeps are the constructor arguments a test may replace. A zero field
// takes the helper's default, so a test names only what it is about.
//
// They are handed in BEFORE construction on purpose: writing them onto a built
// service would be a setter by another name, and the service has none.
type serviceDeps struct {
	// concrete replaces the mock auth method registered under methodID.
	concrete authmethod.AuthMethod
	// inviteOnly disables open signup -- the production shape, where
	// AllowCreate is what decides.
	inviteOnly bool
	// codes is the dance code store. Nil for tests that never reach the dance.
	codes DanceCodeStore
	// invitations is the invitation side of an invited dance. Nil for tests
	// whose dances carry no invitation handle.
	invitations InvitationClaimer
	// flags answers whether a built-in method is offered. Nil means every
	// built-in is, the same way license.NewNoop stands in for the seat cap:
	// most tests exercise sign-in flows, not the settings table. Tests that are
	// about the flags pass their own.
	flags AuthMethodFlags
	// wrapMethods, when set, receives the helper's login configuration and
	// returns the one the service reads -- for a test that overrides one answer
	// of the real configuration and defers the rest to it.
	wrapMethods func(AuthMethods) AuthMethods
}

func initServiceWithDeps(
	t *testing.T,
	methodID entity.AuthMethod,
	deps serviceDeps,
) (*Service, *serviceMocks) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mocks := &serviceMocks{
		authMethod:   mock_authmethod.NewMockAuthMethod(ctrl),
		otpVerifier:  mock_auth.NewMockOTPVerifier(ctrl),
		otpRequester: mock_auth.NewMockOTPRequester(ctrl),
	}
	mocks.authMethod.EXPECT().
		MethodID().
		Return(methodID).
		AnyTimes()

	method := authmethod.AuthMethod(mocks.authMethod)
	if deps.concrete != nil {
		method = deps.concrete
	}

	txManager := dbtx.NewTxManager(db)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tokenSrv := token.NewService(
		txManager,
		refreshtoken.NewStore(db),
		key,
		tokenIssuer,
		"kid-1",
	)

	// Each service gets its own JWT config copy: parallel subtests tweak fields
	// like RefreshTokenGracePeriod, and sharing the package-level cfg.JWT pointer
	// would race under -race.
	jwtCfg := cfg.JWT

	flags := deps.flags
	if flags == nil {
		flags = authflags.NewAllEnabled()
	}

	var methods AuthMethods = authmethod.NewAuthMethods(cfg, []authmethod.AuthMethod{method})
	if deps.wrapMethods != nil {
		methods = deps.wrapMethods(methods)
	}

	return NewService(
		&jwtCfg,
		txManager,
		user.NewService(
			txManager,
			users.NewStore(db),
			useridentities.NewStore(db),
			newTestAuditPublisher(t),
			tokenSrv,
			license.NewNoop(), // auth tests do not exercise the seat cap
			// allowOpenSignup mirrors the local/dev config by default: a plain
			// exchange of an unknown user provisions a guest, so login tests need
			// no invitation. Invited-dance tests pass false to get the production
			// shape, where AllowCreate is what decides.
			!deps.inviteOnly,
			loginProviders,
		),
		distributedlock.NewStore(valkey),
		blacklisttoken.NewStore(valkey),
		methods,
		tokenSrv,
		newTestAuditPublisher(t),
		mocks.otpVerifier,
		mocks.otpRequester,
		authcredentials.NewStore(db),
		flags,
		deps.codes,
		// The same lifetime the dance stores in this package are built with.
		cfg.Auth.DanceStateTTL(),
		deps.invitations,
	), mocks
}

// newTestAuditPublisher builds the audit publisher backed by the test DB's goque
// queue. These tests exercise auth flows, not the audit drain, so events enqueue
// durably and nothing processes them — the tests here don't assert on audit_log
// rows.
func newTestAuditPublisher(t *testing.T) *auditpublisher.Publisher {
	t.Helper()
	storage, err := goque.NewStorage(db)
	require.NoError(t, err)
	return auditpublisher.New(goque.NewTaskQueueManager(storage))
}

// exchangeIDTokenMock sets up the Authenticate expectation for the ID-token
// exchange flow and returns the identity the mocked provider resolves. Tests use
// it to mint a token pair via srv.ExchangeIDToken. The returned value carries
// the resolved email so callers can assert on the provisioned user.
func exchangeIDTokenMock(mocks *serviceMocks, times int) *entity.OAuthProviderUserInfo {
	oauthUser := &entity.OAuthProviderUserInfo{
		ID:    xuuid.NewString(),
		Email: xuuid.NewString() + "_alice@example.com",
		Name:  "alice",
	}
	mocks.authMethod.EXPECT().
		Authenticate(gomock.Any(), "id-token").
		Return(&entity.OAuthIDTokenClaims{
			Subject: oauthUser.ID,
			Email:   oauthUser.Email,
			Name:    oauthUser.Name,
		}, nil).
		Times(times)

	return oauthUser
}

// loginProviders resolves the provider names these tests sign in with. See
// testdbutils.SeedLoginProviders for why the rows have to exist at all.
var loginProviders *testdbutils.LoginProviders
