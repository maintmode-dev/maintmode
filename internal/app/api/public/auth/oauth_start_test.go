package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// startRequest drives the /start handler with the given provider path param and
// returns the recorder, so a test can read both the redirect and the cookie.
func startRequest(t *testing.T, impl *Implementation) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	c := danceContext(t, rec, string(entity.AuthMethodGoogle))

	require.NoError(t, impl.StartOAuthDance(c))

	return rec
}

// danceContext builds an echo context carrying the {provider} path value.
func danceContext(t *testing.T, rec *httptest.ResponseRecorder, provider string) *echo.Context {
	t.Helper()

	return echotest.ContextConfig{
		Request:    httptest.NewRequest(http.MethodGet, "/login/oauth/"+provider+"/start", http.NoBody),
		Response:   rec,
		PathValues: echo.PathValues{{Name: "provider", Value: provider}},
	}.ToContext(t)
}

func TestStartRedirectsToTheProvider(t *testing.T) {
	impl := initDanceImpl(t)

	rec := startRequest(t, impl)

	require.Equal(t, http.StatusFound, rec.Code)

	target, err := url.Parse(rec.Header().Get(echo.HeaderLocation))
	require.NoError(t, err)

	assert.Equal(t, "https", target.Scheme)
	assert.Equal(t, "accounts.google.com", target.Host)

	q := target.Query()
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, testClientID, q.Get("client_id"))
	assert.Equal(t, testRedirectURI, q.Get("redirect_uri"))
	assert.Equal(t, "openid email profile", q.Get("scope"))
	assert.NotEmpty(t, q.Get("state"))

	// PKCE. The challenge must be the S256 transform, never the verifier itself
	// ("plain"), which would make the whole exercise decorative.
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	assert.NotEmpty(t, q.Get("code_challenge"))

	// The verifier is the secret half and must never leave this service.
	assert.NotContains(t, rec.Header().Get(echo.HeaderLocation), "code_verifier")
}

// TestStartSetsBothDanceCookies is the assertion that would otherwise only fail
// in production.
//
// Caddy serves this backend under `handle_path /auth/*`, which STRIPS the
// prefix: the handler sees /api/v1/... while the browser's URL space is
// /auth/api/v1/... A cookie scoped to the path the handler sees is never sent
// back on the callback, and no handler-level test would notice because there is
// no Caddy in one. So the Path must be the external value from config
// (app.oauth_cookie_path), and this pins that it reaches the cookie unaltered
// rather than being replaced by the mounted route.
func TestStartSetsBothDanceCookies(t *testing.T) {
	impl := initDanceImpl(t)

	rec := startRequest(t, impl)
	cookies := danceCookies(t, rec)

	require.Len(t, cookies, 2, "the dance needs the state signature and the verifier, and nothing else")

	for _, name := range []string{oauthStateCookie, oauthVerifierCookie} {
		cookie, ok := cookies[name]
		require.True(t, ok, "missing cookie %s", name)

		assert.NotEmpty(t, cookie.Value)
		assert.True(t, cookie.HttpOnly, "a dance cookie must be unreadable from JS")
		assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite,
			"Strict would withhold the cookie on the redirect back from Google, which is exactly when it is needed")
		assert.Equal(t, testCookiePath, cookie.Path,
			"the cookie must carry the configured EXTERNAL path; Caddy strips the /auth prefix before the handler sees it")
		// A real lifetime, not a token one. Collapsing MaxAge to a second makes
		// the browser drop the cookie before the consent screen is done, so
		// every dance dies at the callback — as a 302 that reads as success.
		// The exact number is the service's to choose and is not pinned here.
		assert.Greater(t, cookie.MaxAge, 60,
			"a cookie the browser discards mid-consent breaks every dance")

		// Padded base64 would put "=" in a cookie value, which http.SetCookie
		// does not encode and c.Cookie does not decode.
		assert.NotContains(t, cookie.Value, "=")
	}
}

// TestStartCookieCarriesTheSignatureNotTheState is the one that stops a later
// "simplification" of the cookie down to the value it protects.
//
// The provider is sent the plaintext state and the browser holds only its
// signature; that asymmetry IS the binding. A cookie holding the state itself
// would hand whoever reads the redirect URL both halves at once, while every
// round-trip test stayed green.
func TestStartCookieCarriesTheSignatureNotTheState(t *testing.T) {
	impl := initDanceImpl(t)

	rec := startRequest(t, impl)

	target, err := url.Parse(rec.Header().Get(echo.HeaderLocation))
	require.NoError(t, err)

	state := target.Query().Get("state")
	require.NotEmpty(t, state)

	cookies := danceCookies(t, rec)
	stateCookie := cookies[oauthStateCookie]
	require.NotNil(t, stateCookie)

	assert.NotContains(t, stateCookie.Value, state,
		"the cookie must carry a signature over the state, never the state itself")

	// And it must be a signature the callback will accept. Asked by driving the
	// real callback rather than by reaching into internals: a start that mints a
	// signature its own callback refuses is the failure worth catching, and only
	// the round trip proves it does not happen.
	_, q := redirectResult(t, callbackRequest(t, impl, url.Values{
		"code":  {"provider-auth-code"},
		"state": {state},
	}, danceRun{state: state, signature: stateCookie.Value, verifier: cookies[oauthVerifierCookie].Value}))

	assert.NotEmpty(t, q.Get("code"), "the callback must accept the signature /start issued")
	assert.Empty(t, q.Get("error"))
}

// TestStartSecureFlagFollowsTheRedirectScheme pins the attribute whose earlier
// version shipped a real hole.
//
// It used to be !IsDev(), with a comment claiming IsDev() is false only in prod.
// That is wrong: IsDev() covers dev, local and performance_test, and the
// deployed dev stand runs environment: dev behind Caddy on https — so a live
// HTTPS stand was handing out the state signature and the PKCE verifier without
// Secure. Any plain-HTTP request the attacker could provoke to that host would
// have carried both halves of a dance, and there is no HSTS in this deployment
// to fall back on.
//
// Deriving it from the redirect_uri's scheme ties the flag to how the instance
// is actually reached rather than to what its environment is called, so a new
// stand cannot reintroduce the hole by picking a name.
func TestStartSecureFlagFollowsTheRedirectScheme(t *testing.T) {
	tests := map[string]struct {
		redirectURI string
		want        bool
	}{
		"https stand carries Secure": {
			redirectURI: "https://host/auth/api/v1/login/oauth/google/callback",
			want:        true,
		},
		// The case the old gate got wrong: an HTTPS stand that is NOT prod.
		"https dev stand carries Secure too": {
			redirectURI: "https://dev.maintmode.dev/auth/api/v1/login/oauth/google/callback",
			want:        true,
		},
		// Plain HTTP on localhost is the only reason this flag is conditional at
		// all: hard-coding it true makes every local dance fail silently.
		"plain http localhost does not": {
			redirectURI: "http://localhost:9000/auth/api/v1/login/oauth/google/callback",
			want:        false,
		},
		// A misconfigured value fails towards the safe side: a cookie the
		// browser withholds beats one it leaks.
		"an unparseable redirect_uri defaults to Secure": {
			redirectURI: "://not-a-url",
			want:        true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			impl := initDanceImplForRedirectURI(t, tt.redirectURI)

			cookies := danceCookies(t, startRequest(t, impl))
			require.Len(t, cookies, 2)

			for cookieName, cookie := range cookies {
				assert.Equal(t, tt.want, cookie.Secure, cookieName)
			}
		})
	}
}

// TestStartRejectsUnknownProviderBeforeAnyWork covers the ordering that keeps
// the new param route from swallowing its static neighbors. The allow-list runs
// first, and an unknown provider gets a 400 JSON rather than a redirect: there
// is no trusted frontend target yet, and echoing an arbitrary path segment into
// a Location header is how open redirects begin.
func TestStartRejectsUnknownProviderBeforeAnyWork(t *testing.T) {
	impl := initDanceImpl(t)

	for _, provider := range []string{"github", "exchange", "", "../etc"} {
		t.Run("provider="+provider, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c := danceContext(t, rec, provider)

			err := impl.StartOAuthDance(c)
			if err != nil {
				// Echo's error handler is not wired in this bare context, so a
				// returned error is the handler refusing — which is the point.
				assert.NotEmpty(t, err.Error())
				return
			}

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Empty(t, rec.Header().Get(echo.HeaderLocation), "a refusal must never redirect")
			assert.Empty(t, rec.Result().Cookies(), "a refusal must not set a dance cookie")
		})
	}
}

// TestStartIssuesAFreshStatePerCall guards against a constant or reused state,
// which would let one captured callback URL be replayed forever.
func TestStartIssuesAFreshStatePerCall(t *testing.T) {
	impl := initDanceImpl(t)

	seen := map[string]bool{}
	for range 5 {
		rec := startRequest(t, impl)

		target, err := url.Parse(rec.Header().Get(echo.HeaderLocation))
		require.NoError(t, err)

		state := target.Query().Get("state")
		require.NotEmpty(t, state)
		assert.False(t, seen[state], "state must be unique per dance")
		seen[state] = true

		// The verifier is the secret half of PKCE and must never travel in the
		// same channel as anything the provider sees.
		verifier := danceCookies(t, rec)[oauthVerifierCookie]
		require.NotNil(t, verifier)
		assert.False(t, strings.Contains(rec.Header().Get(echo.HeaderLocation), verifier.Value),
			"the pkce verifier must not travel in the redirect it protects")
	}
}
