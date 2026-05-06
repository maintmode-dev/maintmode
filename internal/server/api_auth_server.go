package server

import (
	"net/http"

	"github.com/labstack/echo/v5"

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
	s.authV1Group(v1Gr)
	s.s2sV1Group(v1Gr.Group("/s2s"))
}

// auth V1 API group
func (s *APIAuthServer) authV1Group(gr *echo.Group) {
	gr.Add(http.MethodGet, "/.well-known/jwks.json", s.authImpl.JWKS)
	gr.Add(http.MethodGet, "/login/oauth/google", s.authImpl.GoogleOAuthLogin)
	gr.Add(http.MethodGet, "/login/oauth/google/callback", s.authImpl.GoogleOauthCallback)

	gr.Add(http.MethodPost, "/refresh", s.authImpl.Refresh)

	withAuthorize := gr.Group("",
		middlewares.RequireAccessToken(s.tokenVerifier),
	)
	withAuthorize.Add(http.MethodPost, "/logout", s.authImpl.Logout)
	withAuthorize.Add(http.MethodPost, "/logout/all", s.authImpl.LogoutAll)
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
