package auth

import (
	"cmp"
	"context"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5/echotest"
	valkeyDB "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	testconfigutils "github.com/ruko1202/maintmode/test/utils/config"

	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/app/bootstrap"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/gateways/oidcdiscovery"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
	"github.com/ruko1202/maintmode/internal/services/auth"
	"github.com/ruko1202/maintmode/internal/services/authmethod"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	testbootstraputils "github.com/ruko1202/maintmode/test/utils/bootstrap"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
	authflags "github.com/ruko1202/maintmode/test/utils/mocks/authflags"
)

var (
	db     *sqlx.DB
	valkey *valkeyDB.Client
	cfg    *config.AppConfig
)

func TestMain(m *testing.M) {
	cfg = testconfigutils.LoadAuthConfig()

	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	valkey = testdbconnutils.NewValkeyClient(cfg)
	closer.Add(valkey.Close)

	code := m.Run()

	os.Exit(code)
}

func initImpl(t *testing.T) *Implementation {
	t.Helper()

	return newImpl(t, cfg.Auth, nil, nil)
}

// initImplWithOTPFloor builds the handler with an explicit response floor, so a
// test can assert the floor is applied without waiting out the production one.
func initImplWithOTPFloor(t *testing.T, floor time.Duration) *Implementation {
	t.Helper()

	return newImpl(t, config.Auth{OTPResponseFloor: floor}, nil, nil)
}

// initImplWithSettings builds the handler with its own built-in method flags,
// for the tests about the listing and the OTP gate.
func initImplWithSettings(t *testing.T, settings AuthSettings) *Implementation {
	t.Helper()

	return newImpl(t, cfg.Auth, settings, nil)
}

// initImplWithMethods builds the handler over a login configuration the test
// controls, for the tests about the sign-in button list.
func initImplWithMethods(t *testing.T, methods *authmethod.Methods) *Implementation {
	t.Helper()

	return newImpl(t, cfg.Auth, nil, methods)
}

// initImplWithoutBuiltins builds the handler over an auth service that offers
// no built-in sign-in method, for the refusals a password or an emailed code
// would otherwise lift -- disconnecting the last provider, for one.
func initImplWithoutBuiltins(t *testing.T) *Implementation {
	t.Helper()

	stores, err := bootstrap.NewStores(cfg, db, valkey)
	require.NoError(t, err)

	services := newTestServices(t, stores)
	authSrv := newAuthService(stores, services, noBuiltins{}, services.AuthMethods)

	return New(cfg.Auth, authSrv, services.Token, services.User, services.OTP,
		services.AuthMethods, authflags.NewAllEnabled(), config.App{})
}

// noBuiltins answers every built-in method as not offered.
type noBuiltins struct{}

func (noBuiltins) Enabled(context.Context, entity.AuthMethodName) (bool, error) { return false, nil }

// newImpl builds the handler over the real graph. A nil settings source
// means every built-in is offered; nil methods means the graph's own login
// configuration, which no reloader has filled, so it lists no providers.
//
// The dance's frontend half is left unconfigured, as it is for every handler
// test that is not about the dance: /start refuses at its config gate.
func newImpl(
	t *testing.T,
	authCfg config.Auth,
	settings AuthSettings,
	methods *authmethod.Methods,
) *Implementation {
	t.Helper()

	stores, err := bootstrap.NewStores(cfg, db, valkey)
	require.NoError(t, err)

	services := newTestServices(t, stores)

	// Every built-in offered unless a test says otherwise, so the handler tests
	// that are not about the flags need an explicit permissive source, and the
	// tests that ARE about them override it.
	//
	// It goes on BOTH halves, and that is not belt-and-braces. The handler gate
	// and the service gate are separate reads (§4.3): the handler's decides
	// whether a code is issued, the service's whether one is accepted. Wiring
	// only the handler leaves the service reading the LIVE table, so every test
	// that verifies a code passes or fails according to what the migration
	// happens to seed -- which is how a seed change turns into a red test in a
	// file that has nothing to do with the flags. Tests that override the
	// handler's source deliberately leave the service permissive: they assert
	// on the listing and the 202, never on a redeemed code.
	authSrv := newAuthService(stores, services, authflags.NewAllEnabled(), services.AuthMethods)

	if settings == nil {
		settings = authflags.NewAllEnabled()
	}
	if methods == nil {
		methods = services.AuthMethods
	}

	return New(authCfg, authSrv, services.Token, services.User, services.OTP,
		methods, settings, config.App{})
}

// newAuthService rebuilds the auth service from the real graph, the way
// bootstrap assembles it, replacing only the method flags and the login
// configuration it reads.
//
// Rebuilt rather than patched because the service has no setters: what it
// reads is fixed at construction. The rebuilt service is what the handler
// under test talks to; the rest of the graph keeps the bootstrap instance,
// which is harmless here because neither issuing a token pair nor checking one
// reads the flags or the login configuration.
func newAuthService(
	stores *bootstrap.Stores,
	services *bootstrap.Services,
	flags auth.AuthMethodFlags,
	methods auth.AuthMethods,
) *auth.Service {
	return auth.NewService(
		&cfg.JWT,
		stores.TxManager,
		services.User,
		stores.Locker,
		stores.TokenBlackList,
		methods,
		services.Token,
		services.AuditPublisher,
		services.OTP,
		services.OTP,
		stores.AuthCredentials,
		flags,
		stores.OAuthDance,
		cfg.Auth.DanceStateTTL(),
		services.InvitationClaimer,
	)
}

func issueTokenPair(ctx context.Context, t *testing.T, impl *Implementation) *entity.TokenPair {
	t.Helper()

	c := echotest.ContextConfig{}.ToContext(t)

	tokenPair, err := impl.authSrv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
		Provider: entity.AuthMethodGoogle,
		IDToken:  "stub-id-token",
		ClientIP: c.RealIP(),
	})
	require.NoError(t, err)

	return tokenPair
}

// newTestServices builds the real service graph and then points discovery at a
// resolver that will dial loopback.
//
// These tests stand up stub IdPs on httptest servers, which listen on 127.0.0.1
// -- the address the production resolver's dial guard exists to refuse. Without
// this every dance test fails at discovery with "blocked address", which is the
// guard working, not a bug.
//
// Only the dial POLICY is relaxed, and only for loopback: the substitute keeps
// the same timeout, sanitizer and caches, and still refuses private,
// link-local, metadata and reserved addresses. Nothing else about the graph is
// faked, so what these tests exercise is still the wiring production runs.
func newTestServices(t *testing.T, stores *bootstrap.Stores) *bootstrap.Services {
	t.Helper()

	testbootstraputils.SeedLoginProvidersT(t.Context(), t, db)

	services, err := bootstrap.NewServices(t.Context(), cfg, stores)
	require.NoError(t, err)

	services.OIDCDiscovery = oidcdiscovery.NewAllowingLoopback(10 * time.Second)

	return services
}

// installProviders puts a provider set into the live snapshot the way the
// process does: through a reloader reading a registry.
//
// Tests used to call an installer on Methods directly, which meant that entry
// point and its input type had to be exported for them alone -- nothing in the
// running binary installs a provider except the reloader. Driving the reloader
// instead keeps that surface closed AND exercises the real path: rows in, built
// halves out.
func installProviders(t *testing.T, methods *authmethod.Methods, providers ...testProvider) {
	t.Helper()

	rows := make([]entity.ConfiguredProvider, 0, len(providers))
	for _, in := range providers {
		rows = append(rows, entity.ConfiguredProvider{
			Name:    string(in.ID),
			Enabled: true,
			Settings: integrationkinds.OIDCSettings{
				DisplayName: in.DisplayName,
				// Unreachable by default: building resolves nothing, so a test
				// that only needs a live provider never waits on discovery. A
				// test driving a real exchange points this at its own stub.
				IssuerURL:    cmp.Or(in.IssuerURL, "https://127.0.0.1:1/nowhere"),
				ClientID:     string(in.ID) + "-client-id",
				ClientSecret: in.ClientSecret,
				RedirectURI:  in.RedirectURI,
			},
		})
	}

	authmethod.NewReloader(
		fixedRegistry{rows: rows},
		methods,
		oidcdiscovery.NewAllowingLoopback(10*time.Second),
	).Run(t.Context())
}

// testProvider is one provider as a test wants it to land in the snapshot.
type testProvider struct {
	ID          entity.AuthMethod
	DisplayName string
	// IssuerURL points the built provider at a discovery stub; empty means an
	// address that cannot answer.
	IssuerURL string
	// ClientSecret is needed only by a test that completes a real exchange.
	ClientSecret string
	// RedirectURI is optional; an http one is what clears Secure on the dance
	// cookies, decided by the reloader exactly as it is in the process.
	RedirectURI string
}

type fixedRegistry struct{ rows []entity.ConfiguredProvider }

func (r fixedRegistry) ListLoginProviders(context.Context, string) (
	[]entity.ConfiguredProvider, error,
) {
	return r.rows, nil
}
