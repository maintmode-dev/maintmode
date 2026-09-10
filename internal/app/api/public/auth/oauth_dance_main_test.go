package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/app/bootstrap"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	oidcgw "github.com/ruko1202/maintmode/internal/gateways/oidc"
	"github.com/ruko1202/maintmode/internal/gateways/oidcdiscovery"
	"github.com/ruko1202/maintmode/internal/services/auth"
	"github.com/ruko1202/maintmode/internal/storages/oauthdance"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

const (
	testClientID    = "dance-client-id"
	testRedirectURI = "https://host/auth/api/v1/login/oauth/google/callback"
	testFrontendURL = "https://frontend.example.com"
	// testCookiePath is the EXTERNAL scope, carrying the /auth prefix the proxy
	// strips. Spelled out here rather than read from the stand's config so the
	// assertions pin a value this package controls.
	testCookiePath = "/auth/api/v1/login/oauth"
	// testSecondProvider is the instance that exists only in configuration --
	// no constant in entity names it, which is the point.
	testSecondProvider = "acme"
)

// fakeDanceGateway stands in for Google's token endpoint. The dance's own
// contract with the provider is covered by the gateway package's httptest
// suite; here the interesting behavior is what the handler does with the
// gateway's answer, so this only has to be steerable.
type fakeDanceGateway struct {
	idToken string
	err     error
	calls   int
	gotCode string
	gotVerf string

	// real builds the authorization URL, so the /start tests assert against the
	// URL production would emit rather than one this fake invented. Only the
	// network call is faked; the contract with the provider is not.
	real *oidcgw.Client
}

func (f *fakeDanceGateway) AuthCodeURL(ctx context.Context, state, verifier string) (string, error) {
	return f.real.AuthCodeURL(ctx, state, verifier)
}

func (f *fakeDanceGateway) Exchange(_ context.Context, code, verifier string) (string, error) {
	f.calls++
	f.gotCode = code
	f.gotVerf = verifier

	return f.idToken, f.err
}

// testEnv names the redirect_uri the dance handler tests run against. HTTPS, so
// the cookies carry Secure and the assertions match what production emits.
func testEnv() string { return testRedirectURI }

// danceCookies indexes the LIVE dance cookies a response set, by name.
//
// A cookie the handler expired is deliberately excluded. Clearing a cookie is
// itself a Set-Cookie header, so a test counting the raw slice would pass
// against a handler that set two cookies and immediately cleared both — which
// is exactly what the callback does on every exit.
func danceCookies(t *testing.T, rec *httptest.ResponseRecorder) map[string]*http.Cookie {
	t.Helper()

	live := map[string]*http.Cookie{}

	for _, cookie := range rec.Result().Cookies() {
		switch cookie.Name {
		case oauthStateCookie, oauthVerifierCookie, oauthInvitationCookie:
		default:
			continue
		}

		if cookie.MaxAge < 0 {
			continue
		}

		live[cookie.Name] = cookie
	}

	return live
}

// expiredDanceCookies is the complement: the cookies this response told the
// browser to drop.
func expiredDanceCookies(t *testing.T, rec *httptest.ResponseRecorder) map[string]*http.Cookie {
	t.Helper()

	expired := map[string]*http.Cookie{}

	for _, cookie := range rec.Result().Cookies() {
		if cookie.MaxAge < 0 {
			expired[cookie.Name] = cookie
		}
	}

	return expired
}

// testAgent is a User-Agent unique to one request, used to find its audit rows
// in a shared database. Registered on the test so repeated calls within one test
// share it — an assertion covers the whole test, not one call.
func testAgent(t *testing.T) string {
	t.Helper()

	agent, ok := testAgents.Load(t.Name())
	if !ok {
		agent, _ = testAgents.LoadOrStore(t.Name(), t.Name()+"/"+xuuid.NewString())
	}

	return agent.(string)
}

var testAgents sync.Map

// errProviderRefused is a stand-in provider failure for tests that only care that
// the handler treats the exchange as failed.
var errProviderRefused = errors.New("provider refused the exchange")

func initDanceImpl(t *testing.T) *Implementation {
	t.Helper()

	return initDanceImplForRedirectURI(t, testRedirectURI)
}

// initDanceImplForRedirectURI builds a dance whose cookie attributes follow the
// scheme of the given redirect_uri — which is what decides Secure.
func initDanceImplForRedirectURI(t *testing.T, redirectURI string) *Implementation {
	t.Helper()

	return initDanceImplWith(t, redirectURI, newFakeGateway(t, "stub-id-token", nil))
}

// newFakeGateway builds a gateway whose exchange is faked but whose
// authorization URL is the real one.
func newFakeGateway(t *testing.T, idToken string, err error) *fakeDanceGateway {
	t.Helper()

	return &fakeDanceGateway{
		idToken: idToken,
		err:     err,
		real:    oidcgw.NewClient(danceProviderConfig(t), oidcdiscovery.New()),
	}
}

// testIssuerURL is set by newDiscoveryStub to the loopback issuer the dance
// tests resolve against, so the /start assertions can name the authorization
// endpoint that discovery hands back.
var (
	testIssuerOnce sync.Once
	testIssuerURL  string
)

// newDiscoveryStub serves one well-known document for the whole package.
//
// Endpoints come from discovery now, so the tests need an issuer that answers.
// Google's real endpoint values are served from it, which keeps the /start
// assertions checking the URL production emits rather than one invented here.
// Package-scoped because httptest servers cannot outlive a single test's
// cleanup, and these fixtures are shared across many.
func newDiscoveryStub() string {
	testIssuerOnce.Do(func() {
		mux := http.NewServeMux()
		srv := httptest.NewServer(mux)
		mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{
				"issuer": %q,
				"authorization_endpoint": "https://accounts.google.com/o/oauth2/v2/auth",
				"token_endpoint": "https://oauth2.googleapis.com/token",
				"jwks_uri": "https://www.googleapis.com/oauth2/v3/certs"
			}`, srv.URL)
		})
		testIssuerURL = srv.URL
	})

	return testIssuerURL
}

// danceProviderConfig is the provider block the dance tests run against.
func danceProviderConfig(t *testing.T) config.OIDCProvider {
	t.Helper()

	return config.OIDCProvider{
		DisplayName:  "Google",
		IssuerURL:    newDiscoveryStub(),
		ClientID:     testClientID,
		ClientSecret: "dance-client-secret",
		RedirectURI:  testRedirectURI,
	}
}

// initDanceImplWith builds a dance-enabled handler against the REAL Valkey
// store. The store is where single-use lives, and substituting a fake here
// would quietly retire the property these handler tests most need to hold.
func initDanceImplWith(t *testing.T, redirectURI string, gateway auth.DanceGateway) *Implementation {
	t.Helper()

	stores, err := bootstrap.NewStores(db, valkey)
	require.NoError(t, err)

	services, err := bootstrap.NewServices(t.Context(), cfg, stores)
	require.NoError(t, err)

	provider := danceProviderConfig(t)
	provider.RedirectURI = redirectURI
	providers := config.OauthProviders{
		OIDC: map[string]config.OIDCProvider{string(entity.AuthMethodGoogle): provider},
	}

	// The stand's config leaves the dance credentials commented out, so the
	// registry NewServices built lists no danceable provider. These tests are
	// about a stand that has armed it, so the fixture says so -- the same way it
	// supplies its own gateway rather than reaching for the stand's.
	services.AuthMethods.WithDanceProviders([]string{string(entity.AuthMethodGoogle)})

	// The same store on both sides, exactly as main.go arms it: the auth service
	// parks and redeems the one-time code, the invitation service redeems the
	// handle. Arming only one leaves invited dances silently unable to resolve.
	danceStore := oauthdance.NewStore(valkey, cfg.Auth.DanceStateTTL())
	services.Invitation.WithDanceHandles(danceStore)

	// The signer lives on the service now, so the dance is armed in two places:
	// the service gets the signing key, the handler gets the transport.
	impl := New(cfg.Auth,
		services.Auth.WithDance(cfg.Auth, danceStore,
			map[entity.AuthMethod]auth.DanceGateway{entity.AuthMethodGoogle: gateway}),
		services.Token, services.User, services.OTP)

	return impl.WithOAuthDance(providers, config.App{
		FrontendURL:       testFrontendURL,
		OAuthCallbackPath: cfg.App.OAuthCallbackPath,
		OAuthCookiePath:   testCookiePath,
	})
}

// newDiscoveryStubFor serves a well-known document naming authURL as the
// authorization endpoint, so two instances can be told apart by where /start
// sends the browser.
//
// Unlike newDiscoveryStub this one is per-call: the multi-instance test needs
// two distinct issuers, which a package-level singleton cannot provide.
func newDiscoveryStubFor(t *testing.T, authURL string) string {
	t.Helper()

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{
			"issuer": %q,
			"authorization_endpoint": %q,
			"token_endpoint": "https://oauth2.googleapis.com/token",
			"jwks_uri": "https://www.googleapis.com/oauth2/v3/certs"
		}`, srv.URL, authURL)
	})

	return srv.URL
}

// initMultiInstanceDance builds a dance armed with TWO instances, each with its
// own issuer and its own authorization endpoint.
func initMultiInstanceDance(t *testing.T) *Implementation {
	t.Helper()

	instances := map[string]string{
		string(entity.AuthMethodGoogle): "https://accounts.google.com/o/oauth2/v2/auth",
		testSecondProvider:              "https://sso.acme.example/authorize",
	}

	providers := config.OauthProviders{OIDC: map[string]config.OIDCProvider{}}
	for name, authURL := range instances {
		providers.OIDC[name] = config.OIDCProvider{
			DisplayName:  name,
			IssuerURL:    newDiscoveryStubFor(t, authURL),
			ClientID:     name + "-client-id",
			ClientSecret: name + "-client-secret",
			RedirectURI:  testRedirectURI,
		}
	}

	// The instances go through the config NewServices reads, so the registry is
	// populated the same way production populates it. Arming the danceable set
	// alone would not be enough: Parse refuses a name nothing registered, and
	// /start would answer 400 for an instance the config plainly declares.
	multiCfg := *cfg
	multiCfg.OauthProviders = providers

	stores, err := bootstrap.NewStores(db, valkey)
	require.NoError(t, err)

	services, err := bootstrap.NewServices(t.Context(), &multiCfg, stores)
	require.NoError(t, err)

	gateways := map[entity.AuthMethod]auth.DanceGateway{}
	for name, provider := range providers.OIDC {
		gateways[entity.AuthMethod(name)] = oidcgw.NewClient(provider, services.OIDCDiscovery)
	}

	impl := New(cfg.Auth,
		services.Auth.WithDance(cfg.Auth, oauthdance.NewStore(valkey, cfg.Auth.DanceStateTTL()), gateways),
		services.Token, services.User, services.OTP)

	return impl.WithOIDCProviders(providers).WithOAuthDance(providers, config.App{
		FrontendURL:       testFrontendURL,
		OAuthCallbackPath: cfg.App.OAuthCallbackPath,
		OAuthCookiePath:   testCookiePath,
	})
}
