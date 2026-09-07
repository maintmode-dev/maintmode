package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/app/bootstrap"
	"github.com/ruko1202/maintmode/internal/config"
	googleoauthgw "github.com/ruko1202/maintmode/internal/gateways/googleoauth"
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
	real *googleoauthgw.Client
}

func (f *fakeDanceGateway) AuthCodeURL(state, verifier string) string {
	return f.real.AuthCodeURL(state, verifier)
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
		if cookie.Name != oauthStateCookie && cookie.Name != oauthVerifierCookie {
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
		real:    googleoauthgw.NewClient(danceProviderConfig()),
	}
}

// danceProviderConfig is the provider block the dance tests run against.
func danceProviderConfig() config.GoogleOauthProvider {
	return config.GoogleOauthProvider{
		ClientID:     testClientID,
		ClientSecret: "dance-client-secret",
		RedirectURI:  testRedirectURI,
		// Spelled out because the gateway has no in-code default: these are the
		// values a stand's app.config.yaml carries, and the /start assertions
		// below check the redirect actually points at them.
		AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL: "https://oauth2.googleapis.com/token",
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

	provider := danceProviderConfig()
	provider.RedirectURI = redirectURI

	// The signer lives on the service now, so the dance is armed in two places:
	// the service gets the signing secret, the handler gets the transport.
	impl := New(cfg.Auth,
		services.Auth.WithDance(cfg.Auth, provider.ClientSecret, oauthdance.NewStore(valkey), gateway),
		services.Token, services.User, services.OTP)

	return impl.WithOAuthDance(provider, config.App{
		FrontendURL:       testFrontendURL,
		OAuthCallbackPath: cfg.App.OAuthCallbackPath,
		OAuthCookiePath:   testCookiePath,
	})
}
