package oidcdiscovery_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ruko1202/xhttp/dialguard"
	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/gateways/oidcdiscovery"
)

func testCtx(t *testing.T) context.Context {
	t.Helper()

	return xlog.ContextWithLogger(t.Context(), xlog.NewZapAdapter(zaptest.NewLogger(t)))
}

// discoveryDoc is the smallest well-known document that should satisfy the
// resolver: the four fields it actually reads.
func discoveryDoc(issuer string) string {
	return fmt.Sprintf(`{
		"issuer": %q,
		"authorization_endpoint": "https://idp.example/authorize",
		"token_endpoint": "https://idp.example/token",
		"jwks_uri": "https://idp.example/jwks"
	}`, issuer)
}

// newDiscoveryServer serves body at the well-known path over loopback and
// counts the requests that reach it. Pointing the resolver at an
// httptest.Server exercises the real fetch and parse with no external network,
// the same way the provider suite drives JWKS.
//
// It serves https URLs inside an http document on purpose: the endpoints are
// never dialed by these tests, only validated.
func newDiscoveryServer(t *testing.T, body func(issuer string) string) (url string, hits *atomic.Int64) {
	t.Helper()

	hits = new(atomic.Int64)
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body(srv.URL)))
	})

	return srv.URL, hits
}

func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("reads the endpoints out of the document", func(t *testing.T) {
		t.Parallel()

		url, _ := newDiscoveryServer(t, discoveryDoc)

		got, err := oidcdiscovery.NewAllowingLoopback(10*time.Second).Resolve(testCtx(t), url)
		require.NoError(t, err)
		require.Equal(t, url, got.Issuer)

		authURL, tokenURL := got.Endpoint()
		require.Equal(t, "https://idp.example/authorize", authURL)
		require.Equal(t, "https://idp.example/token", tokenURL)
	})

	// RFC 8414 requires the two to match. Without the check, a document served
	// from one origin could name any issuer it liked and tokens would then be
	// validated against that instead.
	t.Run("refuses a document whose issuer disagrees with the configured one", func(t *testing.T) {
		t.Parallel()

		url, _ := newDiscoveryServer(t, func(string) string {
			return discoveryDoc("https://somewhere.else.example")
		})

		_, err := oidcdiscovery.NewAllowingLoopback(10*time.Second).Resolve(testCtx(t), url)
		require.Error(t, err)
		require.ErrorContains(t, err, "issuer")
	})

	// The client secret goes to the token endpoint. http is permitted for
	// issuer_url outside prod, so a spoofed document reaching this code is not
	// hypothetical.
	t.Run("refuses a non-https endpoint in the document", func(t *testing.T) {
		t.Parallel()

		url, _ := newDiscoveryServer(t, func(issuer string) string {
			return fmt.Sprintf(`{
				"issuer": %q,
				"authorization_endpoint": "https://idp.example/authorize",
				"token_endpoint": "http://idp.example/token",
				"jwks_uri": "https://idp.example/jwks"
			}`, issuer)
		})

		_, err := oidcdiscovery.NewAllowingLoopback(10*time.Second).Resolve(testCtx(t), url)
		require.Error(t, err)
		require.ErrorContains(t, err, "https")
	})

	// A local Keycloak or a test double serves plain http, and there is nothing
	// to intercept on loopback. The exception is deliberately narrow: it is the
	// hostname that permits it, not the environment.

	// The loopback exception is an environment decision, not a per-document one:
	// outside dev there is no configuration in which a plain-http endpoint is
	// the right answer, and permitting it would let a compromised IdP name
	// http://127.0.0.1:<port>/ as its jwks_uri and have this process fetch from
	// inside its own host.

	// SECOND-ORDER SSRF. The issuer an operator types is guarded against
	// internal addresses; the endpoints the DOCUMENT names were not, and they
	// are the URLs this process actually dials.
	//
	// An admin who controls a public issuer passes every check at create, then
	// serves a document naming an internal host. The server posts the client
	// secret to that token_endpoint and fetches signing keys from that
	// jwks_uri -- which is not only an internal-network read but an
	// attacker-steerable source of the keys that decide whether a token is
	// genuine.
	t.Run("refuses an endpoint pointing at an address only the server can reach", func(t *testing.T) {
		t.Parallel()

		// One field at a time, so a guard applied to only some of them fails
		// here rather than passing on the strength of its neighbors.
		for field, internal := range map[string]string{
			"authorization_endpoint": "https://127.0.0.1/authorize",
			"token_endpoint":         "https://169.254.169.254/latest/meta-data",
			"jwks_uri":               "https://10.0.0.5:8200/v1/secret",
		} {
			endpoints := map[string]string{
				"authorization_endpoint": "https://idp.example/authorize",
				"token_endpoint":         "https://idp.example/token",
				"jwks_uri":               "https://idp.example/jwks",
			}
			endpoints[field] = internal

			url, _ := newDiscoveryServer(t, func(issuer string) string {
				return fmt.Sprintf(`{
					"issuer": %q,
					"authorization_endpoint": %q,
					"token_endpoint": %q,
					"jwks_uri": %q
				}`, issuer,
					endpoints["authorization_endpoint"],
					endpoints["token_endpoint"],
					endpoints["jwks_uri"])
			})

			_, err := oidcdiscovery.NewAllowingLoopback(10*time.Second).Resolve(testCtx(t), url)
			require.Errorf(t, err, "%s naming %s must be refused", field, internal)
		}
	})

	// The other side of the rule, and the reason it stops where it does: a
	// legitimate document routinely names endpoints on a different registrable
	// domain than its issuer. This is Google's real shape, measured from the
	// live document -- two of its three endpoints are on googleapis.com while
	// the issuer is accounts.google.com.
	//
	// A same-domain requirement would refuse the preset an operator is most
	// likely to configure, which is why the guard checks the ADDRESS rather than
	// the domain.
	t.Run("allows endpoints on a different domain than the issuer", func(t *testing.T) {
		t.Parallel()

		url, _ := newDiscoveryServer(t, func(issuer string) string {
			return fmt.Sprintf(`{
				"issuer": %q,
				"authorization_endpoint": "https://accounts.google.com/o/oauth2/v2/auth",
				"token_endpoint": "https://oauth2.googleapis.com/token",
				"jwks_uri": "https://www.googleapis.com/oauth2/v3/certs"
			}`, issuer)
		})

		_, err := oidcdiscovery.NewAllowingLoopback(10*time.Second).Resolve(testCtx(t), url)
		require.NoError(t, err, "a cross-domain endpoint is normal, not an attack")
	})

	t.Run("refuses a document missing an endpoint it needs", func(t *testing.T) {
		t.Parallel()

		url, _ := newDiscoveryServer(t, func(issuer string) string {
			return fmt.Sprintf(`{"issuer": %q, "jwks_uri": "https://idp.example/jwks"}`, issuer)
		})

		_, err := oidcdiscovery.NewAllowingLoopback(10*time.Second).Resolve(testCtx(t), url)
		require.Error(t, err)
	})

	t.Run("refuses a malformed document", func(t *testing.T) {
		t.Parallel()

		url, _ := newDiscoveryServer(t, func(string) string { return "{not json" })

		_, err := oidcdiscovery.NewAllowingLoopback(10*time.Second).Resolve(testCtx(t), url)
		require.Error(t, err)
	})

	// A trailing slash in config must not produce a double slash in the
	// well-known path, and must not make the issuer comparison fail.
	t.Run("normalizes a trailing slash on the issuer url", func(t *testing.T) {
		t.Parallel()

		url, hits := newDiscoveryServer(t, discoveryDoc)

		got, err := oidcdiscovery.NewAllowingLoopback(10*time.Second).Resolve(testCtx(t), url+"/")
		require.NoError(t, err)
		require.Equal(t, url, got.Issuer)
		require.Equal(t, int64(1), hits.Load())
	})

	t.Run("caches a success for the life of the resolver", func(t *testing.T) {
		t.Parallel()

		url, hits := newDiscoveryServer(t, discoveryDoc)
		resolver := oidcdiscovery.NewAllowingLoopback(10 * time.Second)
		ctx := testCtx(t)

		for range 3 {
			_, err := resolver.Resolve(ctx, url)
			require.NoError(t, err)
		}

		require.Equal(t, int64(1), hits.Load(), "a cached success must not refetch")
	})

	// An IdP that is down at startup must not be down forever: the instance
	// resolves on a later attempt. This is what lets startup survive an
	// unreachable well-known endpoint.
	t.Run("retries after a failure and succeeds once the idp returns", func(t *testing.T) {
		t.Parallel()

		var up atomic.Bool
		mux := http.NewServeMux()
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)
		mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
			if !up.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(discoveryDoc(srv.URL)))
		})

		// No cool-off: this test is about the retry itself, and a cool-off would
		// only make it wait.
		resolver := oidcdiscovery.NewAllowingLoopback(0)
		ctx := testCtx(t)

		_, err := resolver.Resolve(ctx, srv.URL)
		require.Error(t, err)

		up.Store(true)

		got, err := resolver.Resolve(ctx, srv.URL)
		require.NoError(t, err)
		require.Equal(t, srv.URL, got.Issuer)
	})

	// A fresh failure short-circuits without dialing, so a sustained outage
	// cannot turn every sign-in attempt into a full outbound timeout.
	t.Run("short-circuits during the cool-off after a failure", func(t *testing.T) {
		t.Parallel()

		hits := new(atomic.Int64)
		mux := http.NewServeMux()
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)
		mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
			hits.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
		})

		resolver := oidcdiscovery.NewAllowingLoopback(time.Minute)
		ctx := testCtx(t)

		_, first := resolver.Resolve(ctx, srv.URL)
		require.Error(t, first)
		_, second := resolver.Resolve(ctx, srv.URL)
		require.Error(t, second)

		require.Equal(t, int64(1), hits.Load(), "the second attempt must not reach the idp")
	})
}

// TestResolveIsolatesInstances pins that one dead IdP does not take the others
// with it.
//
// Both the success cache and the failure cool-off are keyed by issuer, and
// "keyed by the wrong thing" is a defect that a single-instance test cannot
// see: with one issuer, a resolver that cooled off globally and one that cooled
// off per issuer behave identically. Multi-instance is the whole point of this
// change, so a failure that quietly disabled every provider would be the worst
// possible regression.
func TestResolveIsolatesInstances(t *testing.T) {
	t.Parallel()

	liveURL, _ := newDiscoveryServer(t, discoveryDoc)

	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(dead.Close)

	// A real cool-off, so the dead instance is genuinely in the short-circuiting
	// state while the live one is asked.
	resolver := oidcdiscovery.NewAllowingLoopback(time.Minute)
	ctx := testCtx(t)

	_, err := resolver.Resolve(ctx, dead.URL)
	require.Error(t, err, "the dead issuer must fail")

	got, err := resolver.Resolve(ctx, liveURL)
	require.NoError(t, err, "a dead issuer must not disable a healthy one")
	require.Equal(t, liveURL, got.Issuer)

	// And the dead one is still refused rather than being fixed by the other's
	// success.
	_, err = resolver.Resolve(ctx, dead.URL)
	require.Error(t, err)
}

// TestResolveSurvivesTheLeaderGivingUp pins that one caller walking away does
// not take the rest of the flight with it.
//
// The winner of a singleflight fetches on behalf of everyone waiting on it. If
// the fetch inherited the winner's cancellation, a browser closing its tab
// would fail the shared request, hand that failure to every waiter with a live
// context, and -- worse -- start a cool-off, so one abandoned sign-in would
// refuse the next ten seconds of them against a healthy IdP.
//
// The race is reproduced explicitly: the handler blocks until the leader has
// been canceled, so there is no timing to hope about.
func TestResolveSurvivesTheLeaderGivingUp(t *testing.T) {
	t.Parallel()

	leaderCanceled := make(chan struct{})
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		<-leaderCanceled
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(discoveryDoc(srv.URL)))
	})

	resolver := oidcdiscovery.NewAllowingLoopback(10 * time.Second)

	leaderCtx, cancelLeader := context.WithCancel(testCtx(t))
	arrived := make(chan struct{})
	leaderDone := make(chan error, 1)
	go func() {
		close(arrived)
		_, err := resolver.Resolve(leaderCtx, srv.URL)
		leaderDone <- err
	}()

	// Let the leader enter the flight, then take its context away.
	<-arrived
	time.Sleep(50 * time.Millisecond)
	cancelLeader()
	close(leaderCanceled)

	require.NoError(t, <-leaderDone, "an abandoned caller must not fail the shared fetch")

	// The follower's own resolve must succeed, and no cool-off may be in force.
	got, err := resolver.Resolve(testCtx(t), srv.URL)
	require.NoError(t, err)
	require.Equal(t, srv.URL, got.Issuer)
}

// TestResolveIsSingleFlighted pins that concurrent first uses collapse into one
// outbound request.
//
// The race is reproduced explicitly rather than hoped for: the handler blocks
// until every caller has arrived, so without single-flight all of them are
// inside the fetch at once and the hit count is N. Removing the single-flight
// must turn this red.
func TestResolveIsSingleFlighted(t *testing.T) {
	t.Parallel()

	const callers = 8

	hits := new(atomic.Int64)
	release := make(chan struct{})
	arrived := make(chan struct{}, callers)

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		arrived <- struct{}{}
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(discoveryDoc(srv.URL)))
	})

	resolver := oidcdiscovery.NewAllowingLoopback(10 * time.Second)
	ctx := testCtx(t)

	var (
		wg    sync.WaitGroup
		start = make(chan struct{})
	)
	errs := make([]error, callers)
	wg.Add(callers)
	for i := range callers {
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = resolver.Resolve(ctx, srv.URL)
		}()
	}
	close(start)

	// One request must have reached the handler; wait for it, then let it
	// answer. If the others were going to arrive too, they are already blocked
	// on the handler by now and the count below catches them.
	<-arrived
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	for _, err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int64(1), hits.Load(), "concurrent first uses must collapse into one fetch")
}

// The production constructor refuses to dial an internal address, and this is
// the only test that drives THAT constructor.
//
// Every other test here uses NewAllowingLoopback, because an httptest.Server
// listens on 127.0.0.1 and the guard would refuse it -- which means the suite
// as a whole says nothing about whether New() carries a guard at all. Deleting
// WithoutInternalHosts from New leaves all of them green.
//
// It is driven through an httptest.Server so the refusal is the ONLY reason the
// fetch fails: the server is listening and would answer a valid discovery
// document, so a passing assertion cannot be a connection refused by something
// else. And loopback is the one range the test constructor hands back, which is
// exactly why this case has to run on the production one.
func TestResolveRefusesAnInternalAddressAtDialTime(t *testing.T) {
	t.Parallel()
	ctx := testCtx(t)

	url, hits := newDiscoveryServer(t, discoveryDoc)

	_, err := oidcdiscovery.New().Resolve(ctx, url)

	require.Error(t, err, "a discovery fetch at a loopback address must be refused")
	require.ErrorIs(t, err, dialguard.ErrBlockedAddress)
	require.Zero(t, hits.Load(),
		"the guard must refuse before the connection, so the server never sees a request")
}

// The guard refuses the ADDRESS, not the URL, which is what makes it a layer
// the validator cannot be: a hostname that resolves to loopback passes every
// string check and is still refused here.
//
// localhost rather than a public name pointing inward, because the second needs
// DNS this suite must not depend on. The mechanism under test is the same one:
// the name is resolved, and the guard judges what came back.
func TestResolveRefusesAHostnameThatResolvesInward(t *testing.T) {
	t.Parallel()
	ctx := testCtx(t)

	url, hits := newDiscoveryServer(t, discoveryDoc)
	named := strings.Replace(url, "127.0.0.1", "localhost", 1)
	require.Contains(t, named, "localhost", "the fixture must exercise a NAME, not a literal")

	_, err := oidcdiscovery.New().Resolve(ctx, named)

	require.ErrorIs(t, err, dialguard.ErrBlockedAddress,
		"a name resolving to loopback must be refused at dial time")
	require.Zero(t, hits.Load())
}

// NewAllowingLoopback gives back loopback and NOTHING else.
//
// It is the constructor every other test here runs on, so if it ever widened
// into "no guard at all" the suite would keep passing while testing a client no
// deployment uses. Loosening the exemption from loopback to the whole list is a
// one-word edit -- and without this test, nothing fails.
//
// The metadata address is the case worth naming: it is the target the guard
// exists for, it is not loopback, and a test fixture must not be able to reach
// it either.
func TestNewAllowingLoopbackExemptsOnlyLoopback(t *testing.T) {
	t.Parallel()
	ctx := testCtx(t)

	resolver := oidcdiscovery.NewAllowingLoopback(10 * time.Second)

	for name, issuer := range map[string]string{
		"cloud metadata": "http://169.254.169.254",
		"private 10/8":   "http://10.255.255.1",
		"cgnat":          "http://100.100.100.200",
	} {
		t.Run("still refuses "+name, func(t *testing.T) {
			t.Parallel()

			_, err := resolver.Resolve(ctx, issuer)

			require.ErrorIs(t, err, dialguard.ErrBlockedAddress,
				"%s is not loopback and must stay blocked in tests too", issuer)
		})
	}
}
