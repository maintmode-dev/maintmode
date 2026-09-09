package oidcdiscovery_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

		got, err := oidcdiscovery.New().Resolve(testCtx(t), url)
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

		_, err := oidcdiscovery.New().Resolve(testCtx(t), url)
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

		_, err := oidcdiscovery.New().Resolve(testCtx(t), url)
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

	t.Run("refuses a document missing an endpoint it needs", func(t *testing.T) {
		t.Parallel()

		url, _ := newDiscoveryServer(t, func(issuer string) string {
			return fmt.Sprintf(`{"issuer": %q, "jwks_uri": "https://idp.example/jwks"}`, issuer)
		})

		_, err := oidcdiscovery.New().Resolve(testCtx(t), url)
		require.Error(t, err)
	})

	t.Run("refuses a malformed document", func(t *testing.T) {
		t.Parallel()

		url, _ := newDiscoveryServer(t, func(string) string { return "{not json" })

		_, err := oidcdiscovery.New().Resolve(testCtx(t), url)
		require.Error(t, err)
	})

	// A trailing slash in config must not produce a double slash in the
	// well-known path, and must not make the issuer comparison fail.
	t.Run("normalizes a trailing slash on the issuer url", func(t *testing.T) {
		t.Parallel()

		url, hits := newDiscoveryServer(t, discoveryDoc)

		got, err := oidcdiscovery.New().Resolve(testCtx(t), url+"/")
		require.NoError(t, err)
		require.Equal(t, url, got.Issuer)
		require.Equal(t, int64(1), hits.Load())
	})

	t.Run("caches a success for the life of the resolver", func(t *testing.T) {
		t.Parallel()

		url, hits := newDiscoveryServer(t, discoveryDoc)
		resolver := oidcdiscovery.New()
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
		resolver := oidcdiscovery.NewWithCoolOff(0)
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

		resolver := oidcdiscovery.NewWithCoolOff(time.Minute)
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
	resolver := oidcdiscovery.NewWithCoolOff(time.Minute)
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

	resolver := oidcdiscovery.New()

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

	resolver := oidcdiscovery.New()
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
