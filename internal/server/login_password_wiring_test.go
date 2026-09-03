package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/config/buildmeta"
)

// TestLoginPasswordWiring pins that the break-glass route exists at the path an
// operator will type, and that it is reachable without a token.
//
// It drives the REAL authPublicV1Group rather than a hand-built echo tree. That
// distinction is the entire value of the test: a fixture of my own would prove
// my fixture works and would keep passing if the production registration moved
// the path, changed the method, or mounted the route on a token-gated group.
//
// Every other test of this endpoint calls the handler in-process, so none of
// them can see a routing mistake. For an ordinary endpoint that gap surfaces
// the first time someone opens the app. For break-glass it surfaces during the
// outage it exists to resolve — which is the worst possible moment to discover
// a typo in a path.
//
// The handler is not exercised here (s.handlers is nil, so reaching it would
// panic); what is asserted is that the router RESOLVES the request to a
// registered route rather than answering 404 or 405. Echo decides that before
// any handler runs.
func TestLoginPasswordWiring(t *testing.T) {
	t.Parallel()

	// The rate limiter needs a Valkey client, and building one here would make
	// this a integration test of the limiter rather than of the routing. The
	// limiter's presence is covered by the registration comment and by the
	// alert's 429 exclusion; what cannot be covered elsewhere is the path.
	s := &APIServer{}

	e := echo.New()
	// A recovery middleware turns the nil-handler panic into a 500, so a
	// resolved route is observable as "not 404/405" without a real handler.
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = c.NoContent(http.StatusInternalServerError)
				}
			}()
			return next(c)
		}
	})

	gr := e.Group("/api/v1")
	s.authPublicV1Group(gr, config.LocalEnvironment, &buildmeta.AppBuildMeta{AppName: "maintmode"})

	post := func(t *testing.T, path string) int {
		t.Helper()

		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		return rec.Code
	}

	t.Run("the documented path resolves to a registered route", func(t *testing.T) {
		t.Parallel()

		code := post(t, "/api/v1/login/password")
		require.NotEqual(t, http.StatusNotFound, code,
			"POST /api/v1/login/password must be registered — this is the path an operator recovering an instance will use")
		require.NotEqual(t, http.StatusMethodNotAllowed, code,
			"the route must accept POST")
	})

	t.Run("the route is not behind the access-token gate", func(t *testing.T) {
		t.Parallel()

		// No Authorization header is sent. A 401 would mean break-glass had been
		// mounted on a token-gated group, which makes it useless: its whole
		// purpose is to work when nobody can obtain a token.
		require.NotEqual(t, http.StatusUnauthorized, post(t, "/api/v1/login/password"))
	})

	t.Run("a GET is refused", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/api/v1/login/password", http.NoBody)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Credentials must never be accepted from a query string, where they
		// would land in access logs and browser history.
		require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	})
}
