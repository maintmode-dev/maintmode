package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/config/buildmeta"
	"github.com/ruko1202/maintmode/internal/entity"
)

// revokedToken is the bearer token stubChecker reports as revoked. Any other
// token stubVerifier knows is active.
const revokedToken = "revoked"

// stubChecker stands in for the auth service's active-token checks: the token
// named revokedToken is revoked, every other one is active and keeps admin
// roles. It counts the read-path checks so a test can tell the gate ran rather
// than inferring it from the status alone.
type stubChecker struct {
	readChecks atomic.Int64
}

func (c *stubChecker) EnsureActiveToken(_ context.Context, token string) ([]entity.Role, error) {
	if token == revokedToken {
		return nil, apperr.ErrInvalidAccessToken
	}
	return []entity.Role{entity.RoleAdmin}, nil
}

func (c *stubChecker) EnsureNotRevoked(_ context.Context, token string) error {
	c.readChecks.Add(1)
	if token == revokedToken {
		return apperr.ErrInvalidAccessToken
	}
	return nil
}

// allowAll authorizes every scenario, so RBAC never masks what the token gate
// did: a 401 can only have come from the gate.
type allowAll struct{}

func (allowAll) Allow(context.Context, []entity.Role, entity.AuthzScenario) (bool, error) {
	return true, nil
}

// publicReads are the GET routes under /api/v1 that are deliberately reachable
// without an access token. Everything else under /api/v1 and /ui/v1 is
// token-gated and must refuse a revoked token. A new public read has to be
// added here on purpose; a new gated group mounted without the revocation check
// fails the test.
var publicReads = map[string]bool{
	"/api/v1/.well-known/jwks.json":          true,
	"/api/v1/auth/providers":                 true,
	"/api/v1/login/oauth/:provider/start":    true,
	"/api/v1/login/oauth/:provider/callback": true,
	"/api/v1/users/invitations/preview":      true,
}

// TestTokenGateRefusesRevokedTokenOnEveryRead pins that a revoked access token
// cannot READ either: every token-gated GET route the real BindRouters
// registers, under /api/v1 and /ui/v1 alike, runs the revocation check and
// answers 401. Reads used to pass on local JWT validation alone, so a logged-out
// or revoked session kept reading until its token expired.
func TestTokenGateRefusesRevokedTokenOnEveryRead(t *testing.T) {
	t.Parallel()

	// A server per request, so each subtest counts only its own checks.
	newServer := func(checker *stubChecker) *APIServer {
		s := NewAPIServer(config.HTTPServer{}, APIServerHandlers{}, APIServerSecurity{
			TokenVerifier: stubVerifier{users: map[string]uuid.UUID{revokedToken: uuid.New()}},
			TokenChecker:  checker,
			Authorizer:    allowAll{},
			License:       unlicensedProvider{},
		}, nil, false)
		s.BindRouters(config.LocalEnvironment, &buildmeta.AppBuildMeta{AppName: "test"})
		return s
	}

	reads, err := newServer(&stubChecker{}).Echo().Router().Routes().FilterByMethod(http.MethodGet)
	require.NoError(t, err)

	gated, public := 0, 0
	for _, route := range reads {
		if publicReads[route.Path] {
			public++
			continue
		}
		if !strings.HasPrefix(route.Path, "/api/v1/") && !strings.HasPrefix(route.Path, "/ui/v1/") {
			continue
		}
		gated++

		t.Run(route.Path, func(t *testing.T) {
			t.Parallel()

			path := route.Path
			for _, param := range route.Parameters {
				path = strings.Replace(path, ":"+param, uuid.NewString(), 1)
			}

			checker := &stubChecker{}
			req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
			req.Header.Set("Authorization", "Bearer "+revokedToken)
			rec := httptest.NewRecorder()
			newServer(checker).Echo().ServeHTTP(rec, req)

			require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
			require.Equal(t, int64(1), checker.readChecks.Load(), "the read was not revocation-checked")
		})
	}

	// Guard against the sweep passing vacuously (a filter that matches nothing),
	// and against the allow-list outliving the routes it excuses.
	require.Greater(t, gated, 10, "expected the token-gated reads of /api/v1 and /ui/v1")
	require.Equal(t, len(publicReads), public, "a publicReads entry no longer matches a route")
}
