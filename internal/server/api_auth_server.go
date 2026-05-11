package server

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/ruko1202/maintmode/internal/app/api/public/audit"

	"github.com/ruko1202/maintmode/internal/app/api/public/auth"
	"github.com/ruko1202/maintmode/internal/app/api/public/roles"
	"github.com/ruko1202/maintmode/internal/config/buildmeta"

	_ "github.com/ruko1202/maintmode/docs" // swagger docs
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/server/middlewares"
)

type APIAuthServer struct {
	*server
	s2s       config.S2SConfig
	authImpl  *auth.Implementation
	rolesImpl *roles.Implementation
	auditImpl *audit.Implementation

	tokenVerifier middlewares.TokenVerifier
	authorizer    middlewares.Authorizer
}

func NewAPIAuthServer(
	cfg config.HTTPServer,
	s2s config.S2SConfig,
	authImpl *auth.Implementation,
	rolesImpl *roles.Implementation,
	auditImpl *audit.Implementation,
	tokenVerifier middlewares.TokenVerifier,
	authorizer middlewares.Authorizer,
	opts ...Option,
) *APIAuthServer {
	return &APIAuthServer{
		server:    newServer(cfg, opts...),
		authImpl:  authImpl,
		rolesImpl: rolesImpl,
		auditImpl: auditImpl,
		s2s:       s2s,

		tokenVerifier: tokenVerifier,
		authorizer:    authorizer,
	}
}

func (s *APIAuthServer) BindRouters(env config.Environment, meta *buildmeta.AppBuildMeta) {
	rootGr := s.e.Group("")
	rootGr.Use(middlewares.BaseAPIMiddlewares(env, meta)...)
	rootGr.RouteNotFound("/*", s.notFoundHandler, middlewares.RequestLoggingMiddleware())

	v1Gr := rootGr.Group("/api/v1")
	s.authV1Group(v1Gr, env)
	s.s2sV1Group(v1Gr.Group("/s2s"))
}

// auth V1 API group
func (s *APIAuthServer) authV1Group(gr *echo.Group, env config.Environment) {
	gr.Add(http.MethodGet, "/.well-known/jwks.json", s.authImpl.JWKS)

	loginOAuthGr := gr.Group("/login/oauth",
		middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{
			Rate:      0.5, // 30 RPM
			Burst:     30,
			ExpiresIn: 3 * time.Minute,
		})),
	)

	loginOAuthGr.Add(http.MethodGet, "/google", s.authImpl.GoogleOAuthLogin)
	loginOAuthGr.Add(http.MethodGet, "/google/callback", s.authImpl.GoogleOauthCallback,
		middlewares.NotAllowedInProd(env),
	)
	loginOAuthGr.Add(http.MethodPost, "/exchange/google", s.authImpl.ExchangeGoogleToken,
		middlewares.NotAllowedInProd(env),
	)

	gr.Add(http.MethodPost, "/refresh", s.authImpl.Refresh)

	withAuthorize := gr.Group("",
		middlewares.RequireAccessToken(s.tokenVerifier),
	)
	withAuthorize.Add(http.MethodPost, "/logout", s.authImpl.Logout)
	withAuthorize.Add(http.MethodPost, "/logout/all", s.authImpl.LogoutAll)
	withAuthorize.Add(http.MethodGet, "/me", s.authImpl.Me)
	withAuthorize.Add(http.MethodGet, "/roles",
		s.rolesImpl.AvailableRoles,
		middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioAuthRolesRead),
	)
	withAuthorize.Add(http.MethodPost, "/roles/assign",
		s.rolesImpl.Assign,
		middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioAuthRolesManage),
	)
	withAuthorize.Add(http.MethodPost, "/roles/revoke",
		s.rolesImpl.Revoke,
		middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioAuthRolesManage),
	)
	withAuthorize.Add(http.MethodGet, "/user/:id/roles",
		s.rolesImpl.ListRoles,
		middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioAuthUserRolesRead),
	)

	withAuthorize.Add(http.MethodGet, "/audit/log",
		s.auditImpl.AuditLog,
		middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioAuthAuditRead),
	)
}

// S2S-only: wrap introspect with service token middleware.
func (s *APIAuthServer) s2sV1Group(gr *echo.Group) {
	gr.Use(middlewares.RequireS2SToken(s.s2s))

	gr.Add(http.MethodPost, "/introspect", s.authImpl.Introspect)
}
