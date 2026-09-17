package oauth2_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	oauth2gw "github.com/ruko1202/maintmode/internal/gateways/oauth2"
	"github.com/ruko1202/maintmode/internal/gateways/oauth2/github"
)

// newGithubStub stands in for api.github.com and github.com at once, so a test
// can assert the REAL wire contract rather than a mocked transport -- the same
// choice gateways/license makes.
//
// The handler records what it was asked for, because half of what matters here
// is the request: the token must travel in the Authorization header (where the
// sanitizer redacts it) and never in a query string, and /user/emails must be
// asked for one page of 100 rather than walked.
type githubStub struct {
	server *httptest.Server

	userStatus   int
	userBody     string
	emailsStatus int
	emailsBody   string
	emailsHeader http.Header
	tokenBody    string

	gotUserAuth     string
	gotUserPath     string
	gotUserAccept   string
	gotEmailsAccept string
	gotEmailsAuth   string
	gotEmailsPath   string
	gotTokenAccept  string
	emailsCalls     int
}

func newGithubStub(t *testing.T) *githubStub {
	t.Helper()

	s := &githubStub{
		userStatus:   http.StatusOK,
		userBody:     `{"id": 4242, "login": "octocat", "name": "The Octocat"}`,
		emailsStatus: http.StatusOK,
		emailsBody:   `[{"email":"octocat@example.com","primary":true,"verified":true}]`,
		tokenBody:    `{"access_token":"gho_token","token_type":"bearer","scope":"user:email"}`,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		s.gotTokenAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(s.tokenBody))
	})

	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		s.gotUserAuth = r.Header.Get("Authorization")
		s.gotUserPath = r.URL.RequestURI()
		s.gotUserAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(s.userStatus)
		_, _ = w.Write([]byte(s.userBody))
	})

	mux.HandleFunc("/user/emails", func(w http.ResponseWriter, r *http.Request) {
		s.emailsCalls++
		s.gotEmailsAuth = r.Header.Get("Authorization")
		s.gotEmailsPath = r.URL.RequestURI()
		s.gotEmailsAccept = r.Header.Get("Accept")

		for k, vs := range s.emailsHeader {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(s.emailsStatus)
		_, _ = w.Write([]byte(s.emailsBody))
	})

	s.server = httptest.NewServer(mux)
	t.Cleanup(s.server.Close)

	return s
}

// client points the REAL GitHub vendor at the stub server.
//
// Nothing about the vendor is substituted: the endpoints are credentials now,
// so a test redirects them the same way a deployment does. What these tests
// exercise is therefore GitHub's own identity rule, not a stand-in for it.
func (s *githubStub) client() *oauth2gw.Client {
	return oauth2gw.NewClient(s.credentials(), github.New(s.server.URL))
}

func (s *githubStub) credentials() entity.OAuth2Credentials {
	return entity.OAuth2Credentials{
		DisplayName:  "GitHub",
		ClientID:     "Iv1.client",
		ClientSecret: "secret",
		RedirectURI:  "https://example.com/auth/api/v1/login/oauth/github/callback",
		AuthorizeURL: s.server.URL + "/login/oauth/authorize",
		TokenURL:     s.server.URL + "/login/oauth/access_token",
	}
}

// TestAuthCodeURL pins the redirect /start hands the browser.
//
// Built by the gateway rather than the handler for the same reason the OIDC one
// is: the client id, redirect URI and scopes then have exactly one home.
func TestAuthCodeURL(t *testing.T) {
	t.Parallel()

	stub := newGithubStub(t)

	got, err := stub.client().AuthCodeURL(t.Context(), "the-state", "the-verifier")
	require.NoError(t, err)

	assert.Contains(t, got, stub.server.URL+"/login/oauth/authorize")
	assert.Contains(t, got, "client_id=Iv1.client")
	assert.Contains(t, got, "state=the-state")
	assert.Contains(t, got, "code_challenge_method=S256")
	// The verifier itself must never leave: only its S256 challenge does.
	assert.NotContains(t, got, "the-verifier")
	assert.Contains(t, got, "scope=user%3Aemail")
}

// TestAuthCodeURLDefaultScopes pins the default scope set at ONE scope.
//
// read:user is deliberately absent: GET /user needs no scope on an OAuth App
// token, so requesting it would widen the consent screen while granting access
// to nothing this gateway reads.
func TestAuthCodeURLDefaultScopes(t *testing.T) {
	t.Parallel()

	stub := newGithubStub(t)
	got, err := stub.client().AuthCodeURL(t.Context(), "s", "v")
	require.NoError(t, err)

	assert.Contains(t, got, "scope=user%3Aemail")
	assert.NotContains(t, got, "read%3Auser")
}

// TestExchangeReturnsAccessToken pins the one place GitHub differs from OIDC at
// this layer: what Exchange returns is an opaque access token, not an id_token.
//
// Accept: application/json is asserted because GitHub's token endpoint defaults
// to form-encoded output, and a gateway that forgets it decodes nothing.
func TestExchangeReturnsAccessToken(t *testing.T) {
	t.Parallel()

	stub := newGithubStub(t)

	token, err := stub.client().Exchange(t.Context(), "the-code", "the-verifier")
	require.NoError(t, err)

	assert.Equal(t, "gho_token", token)
	assert.Equal(t, "application/json", stub.gotTokenAccept)
}

// TestExchangeRefusesEmptyToken keeps a malformed success from becoming a
// confusing failure one layer up, the way the OIDC gateway refuses a response
// carrying no id_token.
func TestExchangeRefusesEmptyToken(t *testing.T) {
	t.Parallel()

	stub := newGithubStub(t)
	stub.tokenBody = `{"token_type":"bearer","scope":"user:email"}`

	_, err := stub.client().Exchange(t.Context(), "the-code", "the-verifier")

	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrOAuthExchangeFailed)
}

// TestFetchIdentityHappyPath pins the whole resolution in one pass, including
// the two request-side properties that are easy to lose.
func TestFetchIdentityHappyPath(t *testing.T) {
	t.Parallel()

	stub := newGithubStub(t)

	identity, err := stub.client().FetchIdentity(t.Context(), "gho_token")
	require.NoError(t, err)

	assert.Equal(t, "4242", identity.Subject, "subject is the numeric id, stringified")
	assert.Equal(t, "octocat@example.com", identity.Email)
	assert.Equal(t, "The Octocat", identity.Name)

	// The token rides the Authorization header, which is what the sanitizer
	// redacts. A query-string token would be readable in every access log.
	assert.Equal(t, "Bearer gho_token", stub.gotUserAuth)
	assert.Equal(t, "Bearer gho_token", stub.gotEmailsAuth)

	// The Accept header is GitHub's API versioning mechanism, so dropping it is
	// the kind of drift a gateway test exists to catch.
	assert.Equal(t, "application/vnd.github+json", stub.gotUserAccept)
	assert.Equal(t, "application/vnd.github+json", stub.gotEmailsAccept)

	// And nowhere else. The header assertion above proves the token is SENT
	// correctly; this proves it is not ALSO in the query string, where it would
	// be readable in every access log and Referer.
	assert.NotContains(t, stub.gotEmailsPath, "gho_token")
	assert.NotContains(t, stub.gotUserPath, "gho_token")

	// One page of 100, not a walk.
	assert.Contains(t, stub.gotEmailsPath, "per_page=100")
	assert.Equal(t, 1, stub.emailsCalls)
}

// TestFetchIdentityIgnoresPublicUserEmail is the anti-Grafana case.
//
// /user carries an email field and does NOT report whether it is verified, so
// the value cannot be trusted and is not read at all. A gateway that short-
// circuits on it would skip the verification check entirely for every account
// whose email happens to be public -- silently, and for the majority of users.
func TestFetchIdentityIgnoresPublicUserEmail(t *testing.T) {
	t.Parallel()

	stub := newGithubStub(t)
	stub.userBody = `{"id": 7, "login": "octocat", "name": "Octo", "email": "public@example.com"}`
	stub.emailsBody = `[{"email":"verified@example.com","primary":true,"verified":true}]`

	identity, err := stub.client().FetchIdentity(t.Context(), "tok")
	require.NoError(t, err)

	assert.Equal(t, "verified@example.com", identity.Email,
		"the address must come from /user/emails, never from /user")
	assert.Equal(t, 1, stub.emailsCalls, "/user/emails must be consulted even when /user carries an email")
}

// TestFetchIdentityPrivateEmail is the case the ticket names: a user whose
// address is private gets no email from /user at all.
func TestFetchIdentityPrivateEmail(t *testing.T) {
	t.Parallel()

	stub := newGithubStub(t)
	stub.userBody = `{"id": 9, "login": "ghost", "name": "Ghost", "email": null}`
	stub.emailsBody = `[
		{"email":"secondary@example.com","primary":false,"verified":true},
		{"email":"primary@example.com","primary":true,"verified":true}
	]`

	identity, err := stub.client().FetchIdentity(t.Context(), "tok")
	require.NoError(t, err)

	assert.Equal(t, "primary@example.com", identity.Email)
}

// TestFetchIdentityStopsAtFirstMatch pins the break Grafana's loop does not
// have.
//
// Their implementation keeps assigning and takes the LAST primary record. With
// GitHub's API that is unobservable today, but "last of an unordered list" is
// not a rule anyone chose, and this asserts the one that was.
func TestFetchIdentityStopsAtFirstMatch(t *testing.T) {
	t.Parallel()

	stub := newGithubStub(t)
	stub.emailsBody = `[
		{"email":"first@example.com","primary":true,"verified":true},
		{"email":"second@example.com","primary":true,"verified":true}
	]`

	identity, err := stub.client().FetchIdentity(t.Context(), "tok")
	require.NoError(t, err)

	assert.Equal(t, "first@example.com", identity.Email)
}

// TestFetchIdentityRefusesUnusableEmail covers every shape of "no address this
// backend may trust".
//
// Email is the key that matches an invitation to a person, so an unverified or
// non-primary address is refused rather than accepted with a caveat. There is
// deliberately no fallback: not the public /user email, not the first merely
// verified address, not <login>@users.noreply.github.com.
func TestFetchIdentityRefusesUnusableEmail(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		emails string
	}{
		{
			name:   "primary but unverified",
			emails: `[{"email":"a@example.com","primary":true,"verified":false}]`,
		},
		{
			name:   "verified but not primary",
			emails: `[{"email":"a@example.com","primary":false,"verified":true}]`,
		},
		{
			name:   "no addresses at all",
			emails: `[]`,
		},
		{
			name:   "a primary record carrying an empty address",
			emails: `[{"email":"","primary":true,"verified":true}]`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stub := newGithubStub(t)
			stub.emailsBody = tc.emails

			_, err := stub.client().FetchIdentity(t.Context(), "tok")

			require.Error(t, err)
			assert.ErrorIs(t, err, apperr.ErrGithubEmailUnusable)
		})
	}
}

// TestFetchIdentityRefusesUnusableSubject guards the identity key.
//
// An identity with no subject would collide in
// user_identities(provider, subject) with every other such identity -- so a
// missing or zero id is refused rather than allowed to default to "".
func TestFetchIdentityRefusesUnusableSubject(t *testing.T) {
	t.Parallel()

	for _, body := range []string{
		`{"login":"octocat","name":"Octo"}`,
		`{"id": 0, "login":"octocat","name":"Octo"}`,
	} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			stub := newGithubStub(t)
			stub.userBody = body

			_, err := stub.client().FetchIdentity(t.Context(), "tok")

			require.Error(t, err)
			assert.ErrorIs(t, err, apperr.ErrGithubIdentityUnusable)

			// The guard's PLACEMENT, not just its existence. Its comment claims
			// the refusal happens before the second call, because an identity
			// with no subject cannot be stored whatever its email turns out to
			// be. Moved below resolveEmail the guard still refuses -- and spends
			// an outbound request to reach the same answer.
			assert.Zero(t, stub.emailsCalls,
				"a subject-less identity must be refused before the email read")
		})
	}
}

// TestFetchIdentityUpstreamFailures pins that a broken upstream is reported as
// a provider failure, never as an email problem.
//
// A 403 on /user/emails is one refusal whatever caused it -- a missing scope and
// a secondary rate limit are not told apart, by decision. What must not happen
// is either of them surfacing as ErrGithubEmailUnusable, which would tell a user
// their email is unverified when nothing about their email is known.
func TestFetchIdentityUpstreamFailures(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		prepare func(*githubStub)
	}{
		{
			name:    "user endpoint refuses",
			prepare: func(s *githubStub) { s.userStatus = http.StatusUnauthorized; s.userBody = `{}` },
		},
		{
			name:    "user endpoint returns malformed json",
			prepare: func(s *githubStub) { s.userBody = `{"id": ` },
		},
		{
			name:    "emails endpoint refuses",
			prepare: func(s *githubStub) { s.emailsStatus = http.StatusForbidden; s.emailsBody = `{}` },
		},
		{
			name: "emails endpoint refuses under a rate limit",
			prepare: func(s *githubStub) {
				s.emailsStatus = http.StatusForbidden
				s.emailsBody = `{}`
				s.emailsHeader = http.Header{"X-Ratelimit-Remaining": []string{"0"}}
			},
		},
		{
			name:    "emails endpoint returns malformed json",
			prepare: func(s *githubStub) { s.emailsBody = `[{"email":` },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stub := newGithubStub(t)
			tc.prepare(stub)

			_, err := stub.client().FetchIdentity(t.Context(), "tok")

			require.Error(t, err)
			assert.NotErrorIs(t, err, apperr.ErrGithubEmailUnusable,
				"an upstream failure must not be reported as an email problem")
		})
	}
}

// TestFetchIdentityBoundsResponseBody proves the LimitReader is load-bearing
// rather than decorative.
//
// An unbounded decode of a hostile or broken upstream is how one request eats
// the process's memory. The oversized body is deliberately valid JSON up to the
// limit, so the failure can only come from the bound.
func TestFetchIdentityBoundsResponseBody(t *testing.T) {
	t.Parallel()

	stub := newGithubStub(t)

	stub.emailsBody = oversizedEmailsBody()

	_, err := stub.client().FetchIdentity(t.Context(), "tok")

	require.Error(t, err)
	assert.NotErrorIs(t, err, apperr.ErrGithubEmailUnusable)
}

// TestFetchIdentityHonoursContextCancellation pins that the total callback
// budget can actually bind.
//
// The budget is applied by the caller as one derived context across all three
// calls; this asserts the gateway propagates it instead of holding its own
// per-call deadline as the only bound.
func TestFetchIdentityHonoursContextCancellation(t *testing.T) {
	t.Parallel()

	stub := newGithubStub(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := stub.client().FetchIdentity(ctx, "tok")

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled),
		fmt.Sprintf("a canceled context must surface, got %v", err))
}

// TestFetchIdentityIsBoundedByTheTotalBudget pins the budget that spans both
// identity reads.
//
// The fixture matters more than the assertion. An earlier version hung the FIRST
// call, which apiTimeout cuts at 5s -- so the test passed with totalBudget set
// to a minute, and passed with the budget deleted outright. It measured the
// per-call bound while claiming to measure the total.
//
// Here /user is SLOW but answers, and /user/emails hangs. The first read eats
// most of the budget, so the second one has less than its own apiTimeout left --
// and the call ends at the total rather than at 4s+5s. Only a deadline spanning
// both reads can produce that.
func TestFetchIdentityIsBoundedByTheTotalBudget(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	// SLOW but not hung: it answers inside its own per-call bound, so the first
	// read succeeds and eats most of the budget. That is what leaves the second
	// read with less than apiTimeout to work with -- the only arrangement in
	// which the total is the deadline that fires.
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(4 * time.Second):
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": 1, "login": "octocat", "name": "Octo"}`))
	})
	// Blocks on the REQUEST's context rather than a channel the test closes:
	// httptest.Server.Close waits for in-flight handlers, so a handler parked on
	// test cleanup would deadlock against the cleanup meant to free it.
	mux.HandleFunc("/user/emails", func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := oauth2gw.NewClient(entity.OAuth2Credentials{
		ClientID:     "id",
		ClientSecret: "secret",
		RedirectURI:  "https://example.com/cb",
	}, github.New(srv.URL))

	// A deadline far beyond every bound the gateway carries, so whatever stops
	// this call is the gateway's own.
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	start := time.Now()
	_, err := client.FetchIdentity(ctx, "tok")
	elapsed := time.Since(start)

	require.Error(t, err)

	// Strictly below 4s+apiTimeout: that sum is what the per-call bounds alone
	// would allow, so landing under it is something only the total can cause.
	// The earlier version of this test asserted a ceiling both could satisfy,
	// and passed with the budget deleted outright.
	assert.Less(t, elapsed, 8500*time.Millisecond,
		"the call must end at the total budget, not at the per-call sum")
	assert.Greater(t, elapsed, 4*time.Second,
		"the first read must have completed; otherwise this measures apiTimeout again")
}

// TestAPIBaseJoinsPaths guards the misconfiguration Grafana's shape invites.
//
// Their api_url must already point at .../user and "/emails" is concatenated
// onto it. Here the configured value is the API ROOT and sub-resources are
// joined as paths, so a base carrying a trailing slash must not produce "//user".
func TestAPIBaseJoinsPaths(t *testing.T) {
	t.Parallel()

	var gotPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/user" {
			_, _ = w.Write([]byte(`{"id":1,"login":"a","name":"A"}`))
			return
		}
		_, _ = w.Write([]byte(`[{"email":"a@example.com","primary":true,"verified":true}]`))
	}))
	t.Cleanup(srv.Close)

	// The base carries a TRAILING SLASH, which is the point: url.JoinPath on a
	// base ending in "/" is one of the two ways to get a doubled separator, so
	// the client trims it.
	client := oauth2gw.NewClient(entity.OAuth2Credentials{
		ClientID:     "id",
		ClientSecret: "secret",
		RedirectURI:  "https://example.com/cb",
	}, github.New(srv.URL+"/"))

	_, err := client.FetchIdentity(t.Context(), "tok")
	require.NoError(t, err)

	assert.Equal(t, []string{"/user", "/user/emails"}, gotPaths)
}

// TestScopesComeFromTheVendorOnly pins that the consent screen is not
// operator-configurable.
//
// There is no scopes field to set: the credentials carry none, so the only
// source is the vendor. That is deliberate -- an operator cannot know which
// scope a vendor's identity reads need, and a list typed into an admin form
// REPLACES rather than extends, so one that omits user:email would make
// /user/emails unreadable and every sign-in fail on a missing address.
//
// Asserting the absence as well as the presence: a stray scope reaching the URL
// would mean something other than the vendor decided what to ask consent for.
func TestScopesComeFromTheVendorOnly(t *testing.T) {
	t.Parallel()

	stub := newGithubStub(t)
	client := oauth2gw.NewClient(stub.credentials(), github.New(stub.server.URL))

	got, err := client.AuthCodeURL(t.Context(), "s", "v")
	require.NoError(t, err)

	assert.Contains(t, got, "scope=user%3Aemail")
	assert.NotContains(t, got, "read%3Aorg")
}

// TestOversizedFixtureIsValidJSON keeps the oversized-body fixture honest: if
// this fails, that fixture is malformed rather than merely large, and the bound
// test proves nothing.
//
// It validates the REAL string, built by the same helper. An earlier version
// rebuilt a small one by hand, so it validated a different value than the one it
// was guarding.
func TestOversizedFixtureIsValidJSON(t *testing.T) {
	t.Parallel()

	var decoded []map[string]any
	require.NoError(t, json.Unmarshal([]byte(oversizedEmailsBody()), &decoded))
}

// oversizedEmailsBody builds a body that is valid JSON and over the limiter's
// bound, with the padding INSIDE the first object so a decoder must traverse all
// of it before the object closes -- which is what makes truncation observable.
func oversizedEmailsBody() string {
	var sb strings.Builder
	sb.WriteString(`[{"email":"a@example.com","primary":true,"verified":true,"pad":"`)
	sb.WriteString(strings.Repeat("x", (1<<20)+1))
	sb.WriteString(`"}]`)

	return sb.String()
}
