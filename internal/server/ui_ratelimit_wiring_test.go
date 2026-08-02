package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/server/middlewares"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// uiV1MiddlewareCount is the expected length of the /ui/v1 chain: the token
// gate, the license block gate, and the rate limiter, in that order.
const uiV1MiddlewareCount = 3

// unlicensedProvider reports no license, which RequireLicenseNotSuspended
// treats as unrestricted — the self-hosted shape. The license gate is not what
// this test is about; it is present only so the chain is the real one.
type unlicensedProvider struct{}

func (unlicensedProvider) License(context.Context) *entity.License { return nil }

// oneShotStore allows the first request per identifier and denies the rest, so
// the limit is reached after a single call instead of after 300.
type oneShotStore struct {
	seen map[string]bool
}

func newOneShotStore() *oneShotStore {
	return &oneShotStore{seen: make(map[string]bool)}
}

func (s *oneShotStore) Allow(identifier string) (bool, error) {
	if s.seen[identifier] {
		return false, nil
	}
	s.seen[identifier] = true

	return true, nil
}

// stubVerifier stands in for the real token verifier: any non-empty bearer
// token is accepted and resolves to the user named by it.
type stubVerifier struct {
	users map[string]uuid.UUID
}

func (v stubVerifier) VerifyAccessToken(_ context.Context, token string) (*entity.AccessClaims, error) {
	id, ok := v.users[token]
	if !ok {
		return nil, apperr.ErrInvalidAccessToken
	}

	return &entity.AccessClaims{
		UserName:         token,
		UserEmail:        token + "@example.com",
		UserRoles:        []entity.Role{entity.RoleAdmin},
		RegisteredClaims: jwt.RegisteredClaims{Subject: id.String()},
	}, nil
}

// TestUIRateLimitWiring pins the two routing invariants that make the /ui/v1
// limiter mean what it says. Both failure modes it guards against are silent:
// the endpoints keep working, so nothing but this test would notice.
//
//  1. The limiter buckets by user, not by IP. Every httptest request originates
//     from the same address, so under an IP key the second user's request would
//     be denied by the first user's traffic — which is exactly the collective
//     punishment behind one NAT that this ticket set out to avoid.
//
//  2. The limiter runs after RequireAccessToken. Mounted earlier, the extractor
//     finds no user in the context, falls back to the address, and the limiter
//     degrades to the IP-keyed one from (1) without an error or a log line.
func TestUIRateLimitWiring(t *testing.T) {
	t.Parallel()

	const okBody = "handler-reached"

	alice, bob := uuid.New(), uuid.New()
	verifier := stubVerifier{users: map[string]uuid.UUID{"alice": alice, "bob": bob}}

	handler := func(c *echo.Context) error {
		return c.String(http.StatusOK, okBody)
	}

	// The real chain from uiV1Middlewares, built with a limiter whose store denies
	// after a single request — so the ordering under test is production's, not a
	// copy of it that could drift away from production without failing.
	//
	// The limiter is INJECTED rather than patched into the returned slice by
	// index. Patching by index assumes the position it is trying to verify: move
	// the limiter to the front of the chain and the patch overwrites the license
	// gate instead, leaving the real Redis-backed limiter live in the chain. The
	// test would still fail, but on a nil-Redis panic rather than on the
	// assertion, which reports a crash where it should report a misordering.
	newUIGroupWithLicense := func(
		e *echo.Echo, store middleware.RateLimiterStore, license middlewares.LicenseProvider,
	) {
		s := &APIServer{
			server: &server{},
			security: APIServerSecurity{
				TokenVerifier: verifier,
				License:       license,
			},
		}

		chain := s.uiV1Middlewares(middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
			Store:               store,
			IdentifierExtractor: uiRateLimitKey,
		}))
		// Shape check: the injected limiter has to be the last link for the
		// ordering assertions below to mean anything. If someone appends a
		// middleware after it, this fails loudly and names the reason.
		require.Len(t, chain, uiV1MiddlewareCount,
			"the /ui/v1 chain changed shape — re-check that the limiter is still last")

		gr := e.Group("/ui/v1", chain...)
		gr.Add(http.MethodGet, "/calendar", handler)
		// A mutating route so the license gate's behavior is observable: it
		// rejects only mutating requests under a blocked license.
		gr.Add(http.MethodPost, "/calendar", handler)
	}

	newUIGroup := func(e *echo.Echo, store middleware.RateLimiterStore) {
		newUIGroupWithLicense(e, store, unlicensedProvider{})
	}

	doFrom := func(t *testing.T, e *echo.Echo, token, remoteAddr string) int {
		t.Helper()

		req := httptest.NewRequest(http.MethodGet, "/ui/v1/calendar", http.NoBody)
		if token != "" {
			req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
		}
		if remoteAddr != "" {
			req.RemoteAddr = remoteAddr
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		return rec.Code
	}

	// Every request shares httptest's default address unless one is given, which
	// is what makes the per-user assertions below meaningful.
	do := func(t *testing.T, e *echo.Echo, token string) int {
		t.Helper()

		return doFrom(t, e, token, "")
	}

	t.Run("one user's traffic does not throttle another's", func(t *testing.T) {
		t.Parallel()

		e := echo.New()
		newUIGroup(e, newOneShotStore())

		require.Equal(t, http.StatusOK, do(t, e, "alice"), "alice's first request is within her limit")
		require.Equal(t, http.StatusTooManyRequests, do(t, e, "alice"), "alice's second request exceeds it")
		require.Equal(t, http.StatusOK, do(t, e, "bob"),
			"bob shares alice's address but not her bucket — an IP-keyed limiter would deny him here")
	})

	// The position itself, asserted directly.
	//
	// Every case above drives a chain that uiV1Middlewares assembled, so they all
	// move WITH a reordering rather than against it: hoist the limiter to the
	// front and the test's chain is hoisted too, leaving the behavioral
	// assertions green. Only comparing the assembled order against the intended
	// one catches that, which is why this reads the slice instead of serving a
	// request through it.
	t.Run("the limiter is assembled last, behind both gates", func(t *testing.T) {
		t.Parallel()

		s := &APIServer{
			server:   &server{},
			security: APIServerSecurity{TokenVerifier: verifier, License: unlicensedProvider{}},
		}

		// A limiter that records whether it ran before or after the token gate
		// had a chance to populate the context.
		var sawUser bool
		probe := func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c *echo.Context) error {
				_, sawUser = xecho.UserFromEchoCtx(c)

				return next(c)
			}
		}

		chain := s.uiV1Middlewares(probe)
		require.Len(t, chain, uiV1MiddlewareCount,
			"the /ui/v1 chain changed shape — re-check that the limiter is still last")

		e := echo.New()
		gr := e.Group("/ui/v1", chain...)
		gr.Add(http.MethodGet, "/calendar", handler)

		require.Equal(t, http.StatusOK, do(t, e, "alice"))
		require.True(t, sawUser,
			"the limiter ran before RequireAccessToken populated the context, so it would key on the address")
	})

	// The third ordering invariant, and the one the other cases cannot see: the
	// limiter must also sit behind the license gate. A blocked organization's
	// mutating requests are rejected at that gate, so they must never consume
	// limiter budget — otherwise a suspended org could exhaust its own users'
	// buckets with requests that were never going to be served.
	t.Run("a blocked license is rejected before the limiter spends budget", func(t *testing.T) {
		t.Parallel()

		e := echo.New()
		store := newOneShotStore()
		newUIGroupWithLicense(e, store, blockedProvider{})

		req := httptest.NewRequest(http.MethodPost, "/ui/v1/calendar", http.NoBody)
		req.Header.Set(echo.HeaderAuthorization, "Bearer alice")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		require.Equal(t, http.StatusForbidden, rec.Code, "the license gate rejects a mutating request")

		// The decisive assertion: had the limiter run first it would have spent
		// alice's single allowance on a request the license gate then refused,
		// and this GET would come back 429 instead of 200.
		require.Equal(t, http.StatusOK, do(t, e, "alice"),
			"the refused request must not have consumed alice's budget")
	})

	// The 429 body is what the FE actually receives. Echo renders
	// ErrRateLimitExceeded itself, so nothing in this repo pins the shape; if a
	// future error-handler change swallowed it into a 500, only this would notice.
}
