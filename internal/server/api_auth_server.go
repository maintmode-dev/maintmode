package server

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/ruko1202/maintmode/internal/app/api/public/audit"

	"github.com/ruko1202/maintmode/internal/app/api/public/roles"
	"github.com/ruko1202/maintmode/internal/config/buildmeta"
	"github.com/ruko1202/maintmode/internal/services/token"

	"github.com/ruko1202/maintmode/internal/app/api/public/auth"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/server/middlewares"

	_ "github.com/ruko1202/maintmode/docs" // swagger docs
)

type APIAuthServer struct {
	*server
	s2s       config.S2SConfig
	authImpl  *auth.Implementation
	rolesImpl *roles.Implementation
	auditImpl *audit.Implementation
	tokenSrv  *token.Service
}

func NewAPIAuthServer(
	cfg config.HTTPServer,
	s2s config.S2SConfig,
	tokenSrv *token.Service,
	authImpl *auth.Implementation,
	rolesImpl *roles.Implementation,
	auditImpl *audit.Implementation,
	opts ...Option,
) *APIAuthServer {
	return &APIAuthServer{
		server:    newServer(cfg, opts...),
		tokenSrv:  tokenSrv,
		authImpl:  authImpl,
		rolesImpl: rolesImpl,
		auditImpl: auditImpl,
		s2s:       s2s,
	}
}

func (s *APIAuthServer) BindRouters(env config.Environment, meta *buildmeta.AppBuildMeta) {
	rootGr := s.e.Group("")
	rootGr.Use(middlewares.BaseAPIMiddlewares(env, meta)...)
	rootGr.RouteNotFound("/*", echo.NotFoundHandler, middlewares.RequestLoggingMiddleware())

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
		middlewares.JWTAuth(s.tokenSrv),
	)
	withAuthorize.Add(http.MethodPost, "/logout", s.authImpl.Logout)
	withAuthorize.Add(http.MethodPost, "/logout/all", s.authImpl.LogoutAll)
	withAuthorize.Add(http.MethodGet, "/roles", s.rolesImpl.AvailableRoles)
	withAuthorize.Add(http.MethodPost, "/roles/assign", s.rolesImpl.Assign)
	withAuthorize.Add(http.MethodPost, "/roles/revoke", s.rolesImpl.Revoke)
	withAuthorize.Add(http.MethodGet, "/user/:id/roles", s.rolesImpl.ListRoles)

	withAuthorize.Add(http.MethodGet, "/audit", s.auditImpl.AuditLog)
}

// S2S-only: wrap introspect with service token middleware.
func (s *APIAuthServer) s2sV1Group(gr *echo.Group) {
	gr.Use(middlewares.RequireS2SToken(s.s2s))

	gr.Add(http.MethodPost, "/introspect", s.authImpl.Introspect)
}
