package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// danceRun carries what /start handed the browser, so a callback test can
// present the same state and cookies a real browser would.
type danceRun struct {
	state     string
	signature string
	verifier  string
}

// runStart drives /start and collects both halves of the dance, so the callback
// tests exercise the real pairing rather than a hand-built one.
func runStart(t *testing.T, impl *Implementation) danceRun {
	t.Helper()

	rec := startRequest(t, impl)

	target, err := url.Parse(rec.Header().Get(echo.HeaderLocation))
	require.NoError(t, err)

	cookies := danceCookies(t, rec)
	require.Len(t, cookies, 2)

	return danceRun{
		state:     target.Query().Get("state"),
		signature: cookies[oauthStateCookie].Value,
		verifier:  cookies[oauthVerifierCookie].Value,
	}
}

// callbackRequest drives /callback for the google provider.
func callbackRequest(t *testing.T, impl *Implementation, query url.Values, run danceRun) *httptest.ResponseRecorder {
	t.Helper()

	return callbackRequestAs(t, impl, string(entity.AuthMethodGoogle), query, run)
}

// callbackRequestAs drives /callback for an arbitrary {provider} segment.
//
// A danceRun field left empty means the browser sent no such cookie, which is a
// distinct request from sending one with an empty value — see
// callbackWithEmptyStateCookie for that case.
func callbackRequestAs(
	t *testing.T,
	impl *Implementation,
	provider string,
	query url.Values,
	run danceRun,
) *httptest.ResponseRecorder {
	t.Helper()

	cookies := make([]*http.Cookie, 0, 2)
	if run.signature != "" {
		cookies = append(cookies, &http.Cookie{Name: oauthStateCookie, Value: run.signature})
	}

	if run.verifier != "" {
		cookies = append(cookies, &http.Cookie{Name: oauthVerifierCookie, Value: run.verifier})
	}

	return driveCallback(t, impl, provider, query, cookies)
}

// callbackWithEmptyStateCookie sends the state cookie PRESENT but empty.
//
// It cannot be expressed through danceRun, whose empty field means "send no
// cookie at all" — a different request exercising a different branch of
// danceCookieValue.
func callbackWithEmptyStateCookie(
	t *testing.T,
	impl *Implementation,
	query url.Values,
	run danceRun,
	empty bool,
) *httptest.ResponseRecorder {
	t.Helper()

	if !empty {
		return callbackRequest(t, impl, query, run)
	}

	return driveCallback(t, impl, string(entity.AuthMethodGoogle), query, []*http.Cookie{
		{Name: oauthStateCookie, Value: ""},
		{Name: oauthVerifierCookie, Value: run.verifier},
	})
}

// driveCallback is the one place a callback request is built and run.
func driveCallback(
	t *testing.T,
	impl *Implementation,
	provider string,
	query url.Values,
	cookies []*http.Cookie,
) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet,
		"/login/oauth/"+provider+"/callback?"+query.Encode(), http.NoBody)

	// A per-request User-Agent, so an audit assertion can find exactly this
	// request's rows in a database every other test is also writing to. `make
	// tloc` runs the suite twice over one database, so the name alone is not
	// unique either.
	request.Header.Set("User-Agent", testAgent(t))

	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	c := echotest.ContextConfig{
		Request:    request,
		Response:   rec,
		PathValues: echo.PathValues{{Name: "provider", Value: provider}},
	}.ToContext(t)

	require.NoError(t, impl.OAuthDanceCallback(c))

	return rec
}

// redirectResult reads the outcome of a callback redirect: the frontend target
// plus whichever of code/error it carried.
func redirectResult(t *testing.T, rec *httptest.ResponseRecorder) (*url.URL, url.Values) {
	t.Helper()

	require.Equal(t, http.StatusFound, rec.Code, "a callback must always redirect, never answer JSON")

	target, err := url.Parse(rec.Header().Get(echo.HeaderLocation))
	require.NoError(t, err)

	// The named local is what gocritic's evalOrder wants here, not a stylistic
	// preference: it will not accept target.Query() evaluated inside the return.
	query := target.Query()

	return target, query
}

func TestCallbackHappyPathRedirectsWithAOneTimeCode(t *testing.T) {
	impl := initDanceImpl(t)
	run := runStart(t, impl)

	rec := callbackRequest(t, impl, url.Values{
		"code":  {"provider-auth-code"},
		"state": {run.state},
	}, run)

	target, q := redirectResult(t, rec)

	assert.Equal(t, "frontend.example.com", target.Host, "the target comes only from FrontendURL")
	assert.Equal(t, "/auth/oauth/callback", target.Path, "RUK-292 implements this exact route")
	assert.NotEmpty(t, q.Get("code"))
	assert.Empty(t, q.Get("error"))

	// The opaque code must be redeemable exactly once, and for the pair this
	// dance minted.
	pair, err := impl.authSrv.RedeemDanceCode(t.Context(), q.Get("code"))
	require.NoError(t, err)
	require.NotNil(t, pair)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEqual(t, entity.TokenPair{}.SessionID, pair.SessionID)

	// The redirect must carry the opaque code and no token material — a refresh
	// token in a query string outlives the browser session in access logs,
	// history and Referer, and prod's refresh TTL is 720h.
	//
	// Asserted against the raw Location, NOT q.Get("access_token"). A named-key
	// assertion is blind to the key it is not named after: leaking the pair as
	// ?tok=<access token> keeps every q.Get() empty and the test green. Verified
	// by mutation — that is exactly what happened here.
	location := rec.Header().Get(echo.HeaderLocation)
	assert.NotContains(t, location, pair.AccessToken, "no access token may ride the redirect")
	assert.NotContains(t, location, pair.RefreshToken, "no refresh token may ride the redirect")
	assert.NotContains(t, location, pair.SessionID.String(), "not even the session id")
}

// TestCallbackClearsBothCookiesOnEveryExit covers the invariant step 0 exists
// to create.
//
// Asserting it only on the happy path is what the store-based version did, and
// it left every failure branch free to leak a live cookie into the next attempt.
// Each branch is driven separately here for that reason.
func TestCallbackClearsBothCookiesOnEveryExit(t *testing.T) {
	happy := url.Values{"code": {"provider-auth-code"}}

	tests := map[string]struct {
		query   url.Values
		mutate  func(*danceRun)
		wantErr string
	}{
		"success":          {query: happy},
		"provider error":   {query: url.Values{"error": {"access_denied"}}, wantErr: "access_denied"},
		"no state cookie":  {query: happy, mutate: func(r *danceRun) { r.signature = "" }, wantErr: "state_invalid"},
		"forged signature": {query: happy, mutate: func(r *danceRun) { r.signature = "1799999999.forged" }, wantErr: "state_invalid"},
		"no verifier":      {query: happy, mutate: func(r *danceRun) { r.verifier = "" }, wantErr: "state_invalid"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			impl := initDanceImpl(t)
			run := runStart(t, impl)

			query := url.Values{}
			for k, v := range tt.query {
				query[k] = v
			}
			if query.Get("error") == "" {
				query.Set("state", run.state)
			}

			if tt.mutate != nil {
				tt.mutate(&run)
			}

			rec := callbackRequest(t, impl, query, run)

			_, q := redirectResult(t, rec)
			assert.Equal(t, tt.wantErr, q.Get("error"))

			expired := expiredDanceCookies(t, rec)
			assert.Contains(t, expired, oauthStateCookie, "the state cookie must not survive this exit")
			assert.Contains(t, expired, oauthVerifierCookie, "the verifier cookie must not survive this exit")

			// The expiry has to match the original's scope, or the browser keeps
			// the live cookie alongside the tombstone.
			assert.Equal(t, "/auth/api/v1/login/oauth", expired[oauthStateCookie].Path,
				"an expiry with a different Path leaves the original cookie in place")
		})
	}
}

// TestCallbackRefusesAnUnverifiableState is the ticket's own criterion, in the
// form the cookie design can actually deliver.
//
// The store gave single-use through an atomic consume. A signature cannot: it is
// deterministic, so the same (state, cookie) pair verifies as often as it is
// presented inside the window. What IS guaranteed is that a callback which
// cannot present a matching signature does not complete — which covers every
// attacker who has the redirect URL but not the browser.
func TestCallbackRefusesAnUnverifiableState(t *testing.T) {
	valid := url.Values{"code": {"c"}}

	tests := map[string]struct {
		query url.Values
		// mutate rewrites what the browser presents. omitStateParam is a field
		// rather than a check on the case's NAME: the earlier version compared
		// name against a literal, so renaming a case silently changed what it
		// tested.
		mutate         func(*danceRun)
		omitStateParam bool
		// emptyCookie sends the cookie WITH an empty value, which is a different
		// request from sending none. Both must be refused, and the earlier
		// version could not tell them apart: it set signature = "" for both, and
		// the request helper skips a cookie whose value is empty, so the two
		// cases were byte-for-byte identical.
		emptyCookie bool
	}{
		"no cookie at all":     {query: valid, mutate: func(r *danceRun) { r.signature = "" }},
		"empty cookie value":   {query: valid, emptyCookie: true},
		"forged signature":     {query: valid, mutate: func(r *danceRun) { r.signature = "1799999999.AAAA" }},
		"malformed cookie":     {query: valid, mutate: func(r *danceRun) { r.signature = "not-a-cookie" }},
		"state from elsewhere": {query: url.Values{"code": {"c"}, "state": {"never-issued"}}},
		"no state in query":    {query: valid, omitStateParam: true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			impl := initDanceImpl(t)
			run := runStart(t, impl)

			query := url.Values{}
			for k, v := range tt.query {
				query[k] = v
			}
			if _, pinned := tt.query["state"]; !pinned && !tt.omitStateParam {
				query.Set("state", run.state)
			}

			if tt.mutate != nil {
				tt.mutate(&run)
			}

			rec := callbackWithEmptyStateCookie(t, impl, query, run, tt.emptyCookie)

			_, q := redirectResult(t, rec)

			assert.Equal(t, "state_invalid", q.Get("error"))
			assert.Empty(t, q.Get("code"), "a refused state must never yield a redeemable code")
		})
	}
}

// TestCallbackStaleVerifierReachesTheProvider pins §6.2 step 3's asymmetry: an
// ABSENT verifier is refused here, a merely stale one is not.
//
// Nothing enforces the verifier's lifetime server-side, so a wrong-but-present
// verifier must fall through to the token endpoint and surface as
// provider_error. Adding a server-side freshness check would look like a
// tightening and would in fact reclassify a provider failure as a state
// failure, which is a different row in the audit trail and a different message
// to the user. Without this test that change passes.
func TestCallbackStaleVerifierReachesTheProvider(t *testing.T) {
	gateway := newFakeGateway(t, "", errProviderRefused)
	impl := initDanceImplWith(t, testEnv(), gateway)
	run := runStart(t, impl)

	// Present, well-formed, and not the one this dance minted.
	run.verifier = "a-stale-but-perfectly-shaped-verifier"

	_, q := redirectResult(t, callbackRequest(t, impl, url.Values{
		"code":  {"c"},
		"state": {run.state},
	}, run))

	assert.Equal(t, "provider_error", q.Get("error"),
		"a stale verifier is the provider's verdict to give, not ours")
	assert.Equal(t, 1, gateway.calls, "the exchange must actually be attempted")
}

// TestCallbackRefusesAnEmptyAuthorizationCode stops the one branch where
// unvalidated input reaches the network.
//
// A callback with a valid signature but no code used to be handed straight to
// Google's token endpoint, where it could only ever fail. Refusing it here costs
// the attacker their outbound request and costs us nothing.
func TestCallbackRefusesAnEmptyAuthorizationCode(t *testing.T) {
	gateway := newFakeGateway(t, "stub-id-token", nil)
	impl := initDanceImplWith(t, testEnv(), gateway)
	run := runStart(t, impl)

	_, q := redirectResult(t, callbackRequest(t, impl, url.Values{
		"state": {run.state},
	}, run))

	assert.Equal(t, "state_invalid", q.Get("error"))
	assert.Equal(t, 0, gateway.calls, "an empty code must never reach the provider")
}

// TestCallbackRedirectsAreNotCacheable covers the header that keeps a live
// one-time code out of a shared proxy.
//
// A 302 carrying no cache directives is heuristically cacheable (RFC 9111
// §4.2.2), and this one carries a redeemable code in its Location. Every other
// credential-bearing response in this package already sets no-store; asserting
// it here is what stops this one drifting back out of line.
func TestCallbackRedirectsAreNotCacheable(t *testing.T) {
	impl := initDanceImpl(t)

	t.Run("success carries the code", func(t *testing.T) {
		run := runStart(t, impl)

		rec := callbackRequest(t, impl, url.Values{
			"code":  {"provider-auth-code"},
			"state": {run.state},
		}, run)

		require.NotEmpty(t, rec.Header().Get(echo.HeaderLocation))
		assert.Equal(t, "no-store", rec.Header().Get(echo.HeaderCacheControl))
	})

	t.Run("failure, for symmetry", func(t *testing.T) {
		run := runStart(t, impl)
		run.signature = ""

		rec := callbackRequest(t, impl, url.Values{
			"code":  {"c"},
			"state": {run.state},
		}, run)

		assert.Equal(t, "no-store", rec.Header().Get(echo.HeaderCacheControl))
	})
}

// TestCallbackIgnoresARedirectTargetFromTheRequest pins a prohibition rather
// than a behavior, which is why nothing else covers it.
//
// SPEC §6.2 is explicit: the redirect target is built ONLY from
// config.FrontendURL, with no return_to parameter "in this ticket or as a hook
// for a later one". Today the code honors that — frontendURLWith never reads
// the request. But a rule held only by a comment is one refactor from being
// broken, and the failure mode is the worst available here: a return_to hook on
// the success path carries the one-time code, which exchanges for a full token
// pair, to an attacker's host.
//
// So this asserts the negative: whatever the request asks for, the browser goes
// to FrontendURL. It is a regression test for a hole that does not exist yet.
func TestCallbackIgnoresARedirectTargetFromTheRequest(t *testing.T) {
	const attacker = "https://evil.example"

	t.Run("on success, where the one-time code rides along", func(t *testing.T) {
		impl := initDanceImpl(t)
		run := runStart(t, impl)

		target, q := redirectResult(t, callbackRequest(t, impl, url.Values{
			"code":      {"provider-auth-code"},
			"state":     {run.state},
			"return_to": {attacker},
			"redirect":  {attacker},
			"next":      {attacker},
		}, run))

		assert.Equal(t, "frontend.example.com", target.Host,
			"the one-time code must never leave the configured frontend")
		assert.NotEmpty(t, q.Get("code"), "and this must be the success path, or the assertion is vacuous")
	})

	t.Run("on failure", func(t *testing.T) {
		impl := initDanceImpl(t)
		run := runStart(t, impl)
		run.signature = ""

		target, q := redirectResult(t, callbackRequest(t, impl, url.Values{
			"code":      {"c"},
			"state":     {run.state},
			"return_to": {attacker},
		}, run))

		assert.Equal(t, "frontend.example.com", target.Host)
		assert.NotEmpty(t, q.Get("error"), "and this must be the failure path")
	})
}

// TestCallbackRefusesADanceFromAnotherBrowser is the binding, stated as the
// property that survives the rewrite: whoever observes the redirect URL holds
// the state but not the httpOnly cookie.
func TestCallbackRefusesADanceFromAnotherBrowser(t *testing.T) {
	impl := initDanceImpl(t)

	victim := runStart(t, impl)
	attacker := runStart(t, impl)

	// The attacker's own cookies, presented against the victim's state — which
	// is all an observer of the redirect URL could ever have.
	_, q := redirectResult(t, callbackRequest(t, impl, url.Values{
		"code":  {"c"},
		"state": {victim.state},
	}, attacker))

	assert.Equal(t, "state_invalid", q.Get("error"))
	assert.Empty(t, q.Get("code"))
}

// TestCallbackRefusesARewrittenExpiry is the attack the signed exp exists to
// stop, driven end to end through the handler.
//
// The cookie is entirely attacker-controlled. If exp were merely carried rather
// than signed, anyone holding a captured cookie could extend it indefinitely and
// the 10-minute window would be decorative.
func TestCallbackRefusesARewrittenExpiry(t *testing.T) {
	impl := initDanceImpl(t)
	run := runStart(t, impl)

	_, signature, found := strings.Cut(run.signature, ".")
	require.True(t, found)

	run.signature = strconv.FormatInt(time.Now().Add(365*24*time.Hour).Unix(), 10) + "." + signature

	_, q := redirectResult(t, callbackRequest(t, impl, url.Values{
		"code":  {"c"},
		"state": {run.state},
	}, run))

	assert.Equal(t, "state_invalid", q.Get("error"))
}

// TestCallbackRejectsAProviderSwap covers the check that gives the provider its
// place in the signed material.
//
// Every other callback test drives google against a google signature, so this
// branch never evaluates true and deleting it leaves the suite green. A dance is
// pinned to one provider at /start precisely so a callback cannot carry it into
// another — a provider whose id_token this instance would verify against
// different keys.
func TestCallbackRejectsAProviderSwap(t *testing.T) {
	impl := initDanceImpl(t)
	run := runStart(t, impl)

	// A path provider that is NOT the one the dance began with, and one the
	// allow-list rejects too: both halves must refuse, and neither may fall
	// through to the token exchange.
	_, q := redirectResult(t, callbackRequestAs(t, impl, "stub", url.Values{
		"code":  {"c"},
		"state": {run.state},
	}, run))

	assert.Equal(t, "state_invalid", q.Get("error"))
	assert.Empty(t, q.Get("code"), "a provider swap must never yield a redeemable code")
}

func TestCallbackProviderErrors(t *testing.T) {
	tests := map[string]struct {
		providerErr string
		wantError   string
	}{
		// The provider reporting a declined consent screen. Nothing is broken;
		// the user simply starts again.
		"user declined":      {providerErr: "access_denied", wantError: "access_denied"},
		"provider stumbled":  {providerErr: "temporarily_unavailable", wantError: "provider_error"},
		"provider misbehave": {providerErr: "server_error", wantError: "provider_error"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			impl := initDanceImpl(t)
			run := runStart(t, impl)

			_, q := redirectResult(t, callbackRequest(t, impl,
				url.Values{"error": {tt.providerErr}}, run))

			assert.Equal(t, tt.wantError, q.Get("error"))
			assert.Empty(t, q.Get("code"), "a failure must never carry a redeemable code")
		})
	}
}

// TestCallbackProviderExchangeFailure covers the whole downstream leg: a token
// endpoint that refuses must not strand the browser on a JSON error.
func TestCallbackProviderExchangeFailure(t *testing.T) {
	impl := initDanceImplWith(t, testEnv(), newFakeGateway(t, "", errProviderRefused))
	run := runStart(t, impl)

	_, q := redirectResult(t, callbackRequest(t, impl, url.Values{
		"code":  {"c"},
		"state": {run.state},
	}, run))

	assert.Equal(t, "provider_error", q.Get("error"))
	assert.Empty(t, q.Get("code"))
}

// TestCallbackPassesTheCookieVerifierToTheProvider proves PKCE is actually
// wired: the verifier minted at /start, which never reaches the provider, must
// be the one presented at the token endpoint.
func TestCallbackPassesTheCookieVerifierToTheProvider(t *testing.T) {
	gateway := newFakeGateway(t, "stub-id-token", nil)
	impl := initDanceImplWith(t, testEnv(), gateway)
	run := runStart(t, impl)

	callbackRequest(t, impl, url.Values{
		"code":  {"provider-auth-code"},
		"state": {run.state},
	}, run)

	require.Equal(t, 1, gateway.calls)
	assert.Equal(t, "provider-auth-code", gateway.gotCode)
	assert.Equal(t, run.verifier, gateway.gotVerf, "the cookie's verifier must reach the token endpoint")
	assert.NotEqual(t, run.state, gateway.gotVerf, "the verifier is a separate secret from the state")
}

// TestStartAdvertisesTheS256TransformOfTheCookieVerifier closes the PKCE
// downgrade hole: asserting the challenge is merely non-empty cannot tell S256
// from `plain`, and `plain` means the challenge IS the verifier — which defeats
// the point, since whoever intercepts the redirect then holds both halves.
func TestStartAdvertisesTheS256TransformOfTheCookieVerifier(t *testing.T) {
	impl := initDanceImpl(t)

	rec := startRequest(t, impl)

	target, err := url.Parse(rec.Header().Get(echo.HeaderLocation))
	require.NoError(t, err)

	challenge := target.Query().Get("code_challenge")
	require.NotEmpty(t, challenge)
	require.Equal(t, "S256", target.Query().Get("code_challenge_method"))

	verifier := danceCookies(t, rec)[oauthVerifierCookie]
	require.NotNil(t, verifier)

	// Computed here from the RFC rather than by calling the implementation's own
	// helper: a test that reuses the function under test agrees with it by
	// construction, including when both are wrong.
	sum := sha256.Sum256([]byte(verifier.Value))
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(sum[:]), challenge,
		"the advertised challenge must be the S256 transform of the cookie's verifier")
	assert.NotEqual(t, verifier.Value, challenge,
		"a challenge equal to the verifier is the `plain` method, which PKCE exists to avoid")
}
