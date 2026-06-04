package server

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/ruko1202/maintmode/internal/app/api/public/audit"

	"github.com/redis/go-redis/v9"

	"github.com/ruko1202/maintmode/internal/app/api/public/auth"
	"github.com/ruko1202/maintmode/internal/app/api/public/invitations"
	"github.com/ruko1202/maintmode/internal/app/api/public/roles"
	"github.com/ruko1202/maintmode/internal/app/api/public/users"
	"github.com/ruko1202/maintmode/internal/config/buildmeta"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/server/middlewares"
)

type APIAuthServer struct {
	*server
	s2s             config.S2SConfig
	authImpl        *auth.Implementation
	rolesImpl       *roles.Implementation
	usersImpl       *users.Implementation
	invitationsImpl *invitations.Implementation
	auditImpl       *audit.Implementation

	tokenVerifier middlewares.TokenVerifier
	authorizer    middlewares.Authorizer
	redis         *redis.Client
}

func NewAPIAuthServer(
	cfg config.HTTPServer,
	s2s config.S2SConfig,
	authImpl *auth.Implementation,
	rolesImpl *roles.Implementation,
	usersImpl *users.Implementation,
	invitationsImpl *invitations.Implementation,
	auditImpl *audit.Implementation,
	tokenVerifier middlewares.TokenVerifier,
	authorizer middlewares.Authorizer,
	rdb *redis.Client,
	opts ...Option,
) *APIAuthServer {
	return &APIAuthServer{
		server:          newServer(cfg, opts...),
		authImpl:        authImpl,
		rolesImpl:       rolesImpl,
		usersImpl:       usersImpl,
		invitationsImpl: invitationsImpl,
		auditImpl:       auditImpl,
		s2s:             s2s,

		tokenVerifier: tokenVerifier,
		authorizer:    authorizer,
		redis:         rdb,
	}
}

func (s *APIAuthServer) BindRouters(env config.Environment, meta *buildmeta.AppBuildMeta) {
	rootGr := s.e.Group("")
	rootGr.Use(middlewares.BaseAPIMiddlewares(env, meta)...)
	rootGr.RouteNotFound("/*", s.notFoundHandler, middlewares.RequestLoggingMiddleware())

	v1Gr := rootGr.Group("/api/v1")
	s.authV1Group(v1Gr, env, meta)
	s.s2sV1Group(v1Gr.Group("/s2s"))
}

// auth V1 API group
func (s *APIAuthServer) authV1Group(gr *echo.Group, env config.Environment, meta *buildmeta.AppBuildMeta) {
	gr.Add(http.MethodGet, "/.well-known/jwks.json", s.authImpl.JWKS)

	loginOAuthGr := gr.Group("/login/oauth",
		middleware.RateLimiter(NewRateLimiter(meta.AppName, s.redis, s.cfg.RateLimiter)),
	)

	loginOAuthGr.Add(http.MethodGet, "/google", s.authImpl.GoogleOAuthLogin)
	loginOAuthGr.Add(http.MethodGet, "/google/callback", s.authImpl.GoogleOauthCallback,
		middlewares.NotAllowedInProd(env),
	)
	loginOAuthGr.Add(http.MethodPost, "/exchange/google", s.authImpl.ExchangeGoogleToken,
		middlewares.NotAllowedInProd(env),
	)

	gr.Add(http.MethodPost, "/refresh", s.authImpl.Refresh)

	// Public, unauthenticated invitation endpoints. The raw token in the link is
	// the only credential, so they are rate-limited (like login/oauth) to blunt
	// token enumeration. Registered before the :id admin routes so the static
	// "preview"/"accept"/"invitations" paths are not shadowed by a param route.
	invitesGr := gr.Group("/users/invitations",
		middleware.RateLimiter(NewRateLimiter(meta.AppName, s.redis, s.cfg.RateLimiter)),
	)
	invitesGr.Add(http.MethodGet, "/preview", s.invitationsImpl.PreviewInvitation)
	invitesGr.Add(http.MethodPost, "/accept", s.invitationsImpl.AcceptInvitation)

	withAuthorize := gr.Group("",
		middlewares.RequireAccessToken(s.tokenVerifier),
	)
	withAuthorize.Add(http.MethodPost, "/logout", s.authImpl.Logout)
	withAuthorize.Add(http.MethodPost, "/logout/all", s.authImpl.LogoutAll)
	withAuthorize.Add(http.MethodGet, "/me", s.authImpl.Me)
	withAuthorize.Add(http.MethodPost, "/me/providers/:provider/connect", s.authImpl.ConnectProvider)
	withAuthorize.Add(http.MethodDelete, "/me/providers/:provider/disconnect", s.authImpl.DisconnectProvider)
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

	withAuthorize.Add(http.MethodGet, "/users/list",
		s.usersImpl.ListUsers,
		middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioAuthUsersRead),
	)
	withAuthorize.Add(http.MethodPost, "/users/:id/block",
		s.usersImpl.BlockUser,
		middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioAuthUsersManage),
	)
	withAuthorize.Add(http.MethodPost, "/users/:id/unblock",
		s.usersImpl.UnblockUser,
		middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioAuthUsersManage),
	)

	// Admin invitation management. Read uses the users.read scenario; mutating
	// operations use users.manage (same scenarios as block/unblock).
	withAuthorize.Add(http.MethodPost, "/users/invite",
		s.invitationsImpl.Invite,
		middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioAuthUsersManage),
	)
	withAuthorize.Add(http.MethodGet, "/users/invitations",
		s.invitationsImpl.ListInvitations,
		middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioAuthUsersRead),
	)
	withAuthorize.Add(http.MethodPost, "/users/invitations/:id/revoke",
		s.invitationsImpl.RevokeInvitation,
		middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioAuthUsersManage),
	)
	withAuthorize.Add(http.MethodPost, "/users/invitations/:id/resend",
		s.invitationsImpl.ResendInvitation,
		middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioAuthUsersManage),
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
	gr.Add(http.MethodGet, "/users", s.usersImpl.ListUsersS2S)
}
