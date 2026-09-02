package server

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	valkeylib "github.com/redis/go-redis/v9"
	xhttpserver "github.com/ruko1202/xhttp/server"

	"github.com/ruko1202/maintmode/internal/config/buildmeta"

	apiaudit "github.com/ruko1202/maintmode/internal/app/api/public/audit"
	apiauth "github.com/ruko1202/maintmode/internal/app/api/public/auth"
	integrationapi "github.com/ruko1202/maintmode/internal/app/api/public/integration"
	apiinvitations "github.com/ruko1202/maintmode/internal/app/api/public/invitations"
	apimaint "github.com/ruko1202/maintmode/internal/app/api/public/maint"
	apinotifications "github.com/ruko1202/maintmode/internal/app/api/public/notifytargets"
	resourcesapi "github.com/ruko1202/maintmode/internal/app/api/public/resources"
	apiroles "github.com/ruko1202/maintmode/internal/app/api/public/roles"
	userpickerapi "github.com/ruko1202/maintmode/internal/app/api/public/userpicker"
	apiusers "github.com/ruko1202/maintmode/internal/app/api/public/users"
	uiapprovals "github.com/ruko1202/maintmode/internal/app/api/ui/approvals"
	uicalendar "github.com/ruko1202/maintmode/internal/app/api/ui/calendar"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/server/middlewares"
)

// integrationBodyLimitBytes caps request bodies on the integrations group: its
// config/secrets are free-form JSON persisted verbatim, so without a cap a
// single request could bloat the integration_settings row and every subsequent
// resolve. 64 KiB is orders of magnitude above any real transport config.
const integrationBodyLimitBytes int64 = 64 * 1024

// APIServerHandlers holds the per-domain Echo handler implementations served
// by the API server. Adding a new domain means adding a field here, not a new
// constructor argument. With auth folded in-process it carries both
// the core (maint/resource/calendar/notify/userpicker) handlers and the auth
// (auth/roles/users/invitations/audit) handlers.
type APIServerHandlers struct {
	Maint         *apimaint.Implementation
	Resources     *resourcesapi.Implementation
	Calendar      *uicalendar.Implementation
	Approvals     *uiapprovals.Implementation
	Notifications *apinotifications.Implementation
	Integrations  *integrationapi.Implementation
	UserPicker    *userpickerapi.Implementation

	// Auth-module handlers.
	Auth        *apiauth.Implementation
	Roles       *apiroles.Implementation
	Users       *apiusers.Implementation
	Invitations *apiinvitations.Implementation
	Audit       *apiaudit.Implementation
}

// APIServerSecurity holds the security primitives wired into middleware
// chains: local JWT verification, local active-token checks on critical
// mutations, and RBAC scenario authorization.
type APIServerSecurity struct {
	TokenVerifier middlewares.TokenVerifier
	TokenChecker  middlewares.ActiveTokenChecker
	Authorizer    middlewares.Authorizer
	// License is the suspend-gate source: the license service in SaaS mode,
	// license.Noop on self-hosted (its nil license passes every request).
	License middlewares.LicenseProvider
}

type APIServer struct {
	*xhttpserver.Server
	// cfg is kept alongside the embedded server because the rate limiters read
	// their thresholds from it. xhttpserver.Config carries only what it needs
	// to bind a port, so the domain half of config.HTTPServer has to live here.
	cfg      config.HTTPServer
	handlers APIServerHandlers
	security APIServerSecurity
	// valkey backs the rate limiters guarding the public login/oauth and
	// invitation endpoints (keyed by IP) and the /ui/v1 screen group (keyed by
	// user). Each limiter degrades to a per-replica in-memory bucket when valkey
	// is unreachable.
	valkey *valkeylib.Client
}

func NewAPIServer(
	cfg config.HTTPServer,
	handlers APIServerHandlers,
	security APIServerSecurity,
	rdb *valkeylib.Client,
	opts ...xhttpserver.Option,
) *APIServer {
	timeouts := cfg.Timeouts.TimeoutsOrDefault()

	return &APIServer{
		Server: xhttpserver.New(xhttpserver.Config{
			Name: cfg.Name,
			Host: cfg.Host,
			Port: cfg.Port,
			// Socket deadlines for the public port. Without them Echo's own 30s
			// ReadTimeout is the only bound and an idle keep-alive connection is
			// held for free, which is what makes slowloris cheap.
			ReadHeaderTimeout: timeouts.ReadHeader,
			ReadTimeout:       timeouts.Read,
			WriteTimeout:      timeouts.Write,
			IdleTimeout:       timeouts.Idle,
		}, opts...),
		cfg:      cfg,
		handlers: handlers,
		security: security,
		valkey:   rdb,
	}
}

func (s *APIServer) BindRouters(env config.Environment, meta *buildmeta.AppBuildMeta) {
	rootGr := s.Echo().Group("")
	rootGr.Use(middlewares.BaseAPIMiddlewares(env, meta)...)

	rootGr.RouteNotFound("/*", xhttpserver.NotFoundHandler, xhttpserver.RequestLoggingMiddleware())

	// The /api/v1 base group carries NO blanket access-token gate: the auth module
	// exposes public routes (login/oauth, refresh, jwks, invitation preview/accept)
	// that must stay unauthenticated. Each subgroup applies RequireAccessToken (or
	// not) for itself.
	//
	// The license block gate is NOT applied here: the whole auth module — both its
	// public (login/oauth, refresh, invitation accept) and its protected (logout,
	// /me, provider connect, role/user admin) routes — must stay reachable under a
	// blocked license so a member of a blocked org can sign in, manage their
	// session and reach the suspended page. The gate lives inside apiV1Group,
	// covering the business routes only.
	apiV1 := rootGr.Group("/api/v1")
	s.authPublicV1Group(apiV1, env, meta)
	s.apiV1Group(apiV1)
	s.authProtectedV1Group(apiV1)

	// ui/v1 is fully token-gated and also behind the block gate.
	s.uiV1Group(rootGr.Group("/ui/v1",
		s.uiV1Middlewares(NewUIRateLimiter(meta.AppName, s.valkey, s.cfg.UIRateLimiter))...))
}

// uiV1Middlewares builds the middleware chain guarding the /ui/v1 screen group.
// It is a named function rather than an inline argument list because the ORDER
// is the contract: the rate limiter keys on the user that RequireAccessToken
// puts in the context, so it has to come last. Mounted ahead of the token gate
// it would find an empty context, fall back to the remote address, and degrade
// to an IP-keyed limiter — no error, no log, just a limiter that punishes a
// whole office behind one NAT for one person's traffic. TestUIRateLimitWiring
// calls this function for exactly that reason.
//
// The limiter arrives as an argument rather than being built here so that the
// wiring test can supply one whose store denies after a single request. The
// alternative — letting the test overwrite a link by index — silently patches
// the wrong middleware the moment the order changes, which is precisely the
// change the test exists to catch.
func (s *APIServer) uiV1Middlewares(rateLimiter echo.MiddlewareFunc) []echo.MiddlewareFunc {
	return []echo.MiddlewareFunc{
		middlewares.RequireAccessToken(s.security.TokenVerifier),
		middlewares.RequireLicenseNotSuspended(s.security.License),
		rateLimiter,
	}
}

func (s *APIServer) scenarioMW(scenario entity.AuthzScenario) echo.MiddlewareFunc {
	return middlewares.RequireScenario(s.security.Authorizer, scenario)
}

func (s *APIServer) scenarioWithIntrospectMW(scenario entity.AuthzScenario) []echo.MiddlewareFunc {
	return []echo.MiddlewareFunc{
		middlewares.RequireActiveToken(s.security.TokenChecker),
		s.scenarioMW(scenario),
	}
}

// authPublicV1Group registers the auth routes that must NOT sit behind the
// access-token gate: JWKS, the OAuth ID-token exchange, token refresh and the
// unauthenticated invitation preview/accept endpoints. The token in the
// invitation link (and the login/oauth surface) is the only credential, so those
// groups are rate-limited to blunt enumeration.
//
// NOTE: the invitation preview/accept routes live under the STATIC path
// /users/invitations and are registered here, before apiV1Group registers
// /users/:id/... param routes, so the static segment is not shadowed.
func (s *APIServer) authPublicV1Group(gr *echo.Group, _ config.Environment, meta *buildmeta.AppBuildMeta) {
	gr.Add(http.MethodGet, "/.well-known/jwks.json", s.handlers.Auth.JWKS)

	loginOAuthGr := gr.Group("/login/oauth",
		middleware.RateLimiter(NewRateLimiter(meta.AppName, s.valkey, s.cfg.RateLimiter)),
	)
	loginOAuthGr.Add(http.MethodPost, "/exchange/google", s.handlers.Auth.ExchangeGoogleToken)

	gr.Add(http.MethodPost, "/refresh", s.handlers.Auth.Refresh)

	invitesGr := gr.Group("/users/invitations",
		middleware.RateLimiter(NewRateLimiter(meta.AppName, s.valkey, s.cfg.RateLimiter)),
	)
	invitesGr.Add(http.MethodGet, "/preview", s.handlers.Invitations.PreviewInvitation)
	invitesGr.Add(http.MethodPost, "/accept", s.handlers.Invitations.AcceptInvitation)
}

// apiV1Group registers the core (maintenance/resource/notify/userpicker) routes.
// Each subgroup gates itself with RequireAccessToken — the /api/v1 base group
// carries no blanket gate (the auth public routes share it).
//
// The license block gate wraps ONLY these business routes (not the auth module):
// under a blocked license it rejects mutating requests while leaving reads open,
// so a member of a blocked org still sees their data but cannot change it. On
// self-hosted the provider is license.Noop, whose nil license passes everything.
func (s *APIServer) apiV1Group(gr *echo.Group) {
	requireToken := middlewares.RequireAccessToken(s.security.TokenVerifier)
	gr = gr.Group("", middlewares.RequireLicenseNotSuspended(s.security.License))

	// maint API group
	{
		maintAPI := gr.Group("/maintenances", requireToken)
		maintAPI.Add(http.MethodPost, "/create", s.handlers.Maint.CreateDraftMaint,
			s.scenarioMW(entity.AuthzScenarioMaintenanceCreate))
		maintAPI.Add(http.MethodPost, "/:id/edit", s.handlers.Maint.UpdateDraftMaint,
			s.scenarioMW(entity.AuthzScenarioMaintenanceEdit))
		maintAPI.Add(http.MethodPost, "/:id/start", s.handlers.Maint.StartMaint,
			s.scenarioWithIntrospectMW(entity.AuthzScenarioMaintenanceStart)...)
		maintAPI.Add(http.MethodPost, "/:id/cancel", s.handlers.Maint.CancelMaint,
			s.scenarioWithIntrospectMW(entity.AuthzScenarioMaintenanceCancel)...)
		maintAPI.Add(http.MethodPost, "/:id/complete", s.handlers.Maint.CompleteMaint,
			s.scenarioWithIntrospectMW(entity.AuthzScenarioMaintenanceComplete)...)
		maintAPI.Add(http.MethodPost, "/:id/approve", s.handlers.Maint.ApproveMaint,
			s.scenarioWithIntrospectMW(entity.AuthzScenarioMaintenanceApprove)...)
		maintAPI.Add(http.MethodPost, "/:id/steps/:step_id/start", s.handlers.Maint.StartStep,
			s.scenarioWithIntrospectMW(entity.AuthzScenarioMaintenanceStepStart)...)
		maintAPI.Add(http.MethodPost, "/:id/steps/:step_id/complete", s.handlers.Maint.CompleteStep,
			s.scenarioWithIntrospectMW(entity.AuthzScenarioMaintenanceStepComplete)...)
		maintAPI.Add(http.MethodPost, "/:id/steps/:step_id/cancel", s.handlers.Maint.CancelStep,
			s.scenarioWithIntrospectMW(entity.AuthzScenarioMaintenanceStepCancel)...)
		maintAPI.Add(http.MethodGet, "/:id", s.handlers.Maint.GetMaint,
			s.scenarioMW(entity.AuthzScenarioMaintenanceRead))
		maintAPI.Add(http.MethodGet, "/cancel-reasons", s.handlers.Maint.CancelMaintReasons,
			s.scenarioMW(entity.AuthzScenarioMaintenanceRead))
	}

	// resources API group
	{
		resourcesAPI := gr.Group("/resources", requireToken)
		resourcesAPI.Add(http.MethodGet, "", s.handlers.Resources.SearchResources,
			s.scenarioMW(entity.AuthzScenarioResourceRead))
		resourcesAPI.Add(http.MethodGet, "/list", s.handlers.Resources.ListResources,
			s.scenarioMW(entity.AuthzScenarioResourceRead))
	}

	// resource API group
	{
		resourceAPI := gr.Group("/resource", requireToken)
		resourceAPI.Add(http.MethodPost, "/create", s.handlers.Resources.CreateResource,
			s.scenarioMW(entity.AuthzScenarioResourceCreate))
		resourceAPI.Add(http.MethodPost, "/:id/archive", s.handlers.Resources.ArchiveResource,
			s.scenarioMW(entity.AuthzScenarioResourceArchive))
		resourceAPI.Add(http.MethodPost, "/:id/unarchive", s.handlers.Resources.UnarchiveResource,
			s.scenarioMW(entity.AuthzScenarioResourceUnarchive))
		resourceAPI.Add(http.MethodGet, "/:id", s.handlers.Resources.GetResource,
			s.scenarioMW(entity.AuthzScenarioResourceRead))
		resourceAPI.Add(http.MethodPatch, "/:id", s.handlers.Resources.UpdateResource,
			s.scenarioMW(entity.AuthzScenarioResourceEdit))
	}

	// integrations API group — admin-only registry of external-system connections
	// Reads require integration.read; writes require integration.manage.
	// Body-limited: config/secrets are free-form JSON stored verbatim, so this is
	// the one write surface accepting arbitrary payloads — cap it well above any
	// real integration config but below anything that could bloat rows/memory.
	{
		integrationAPI := gr.Group("/integrations", requireToken,
			middleware.BodyLimit(integrationBodyLimitBytes))
		integrationAPI.Add(http.MethodGet, "", s.handlers.Integrations.List,
			s.scenarioMW(entity.AuthzScenarioIntegrationRead))
		integrationAPI.Add(http.MethodPost, "", s.handlers.Integrations.Create,
			s.scenarioMW(entity.AuthzScenarioIntegrationManage))
		integrationAPI.Add(http.MethodGet, "/:kind", s.handlers.Integrations.Get,
			s.scenarioMW(entity.AuthzScenarioIntegrationRead))
		integrationAPI.Add(http.MethodPatch, "/:kind", s.handlers.Integrations.Update,
			s.scenarioMW(entity.AuthzScenarioIntegrationManage))
		integrationAPI.Add(http.MethodPost, "/:kind/toggle", s.handlers.Integrations.Toggle,
			s.scenarioMW(entity.AuthzScenarioIntegrationManage))
	}

	// notifications API group — transports + channel catalog powering the
	// admin picker. The GET endpoints are read-scoped on the maintenance
	// scenario (guest+) since the picker is part of the maintenance edit
	// flow; the transports catalog is a reference list available to any
	// authenticated role. POST manages the channel catalog (the cross-pod
	// source of truth) and is editor-scoped, alongside the other
	// create/edit scenarios.
	{
		notifAPI := gr.Group("/notifications", requireToken)
		notifAPI.Add(http.MethodGet, "/transports", s.handlers.Notifications.GetTransports,
			s.scenarioMW(entity.AuthzScenarioMaintenanceRead))
		notifAPI.Add(http.MethodGet, "/channels", s.handlers.Notifications.GetChannels,
			s.scenarioMW(entity.AuthzScenarioMaintenanceRead))
		notifAPI.Add(http.MethodGet, "/channels/:id", s.handlers.Notifications.GetChannel,
			s.scenarioMW(entity.AuthzScenarioMaintenanceRead))
		notifAPI.Add(http.MethodPost, "/channels", s.handlers.Notifications.CreateChannel,
			s.scenarioMW(entity.AuthzScenarioNotificationChannelCreate))
		notifAPI.Add(http.MethodPatch, "/channels/:id", s.handlers.Notifications.UpdateChannel,
			s.scenarioMW(entity.AuthzScenarioNotificationChannelEdit))
		notifAPI.Add(http.MethodPost, "/channels/:id/archive", s.handlers.Notifications.ArchiveChannel,
			s.scenarioMW(entity.AuthzScenarioNotificationChannelArchive))
		notifAPI.Add(http.MethodPost, "/channels/:id/unarchive", s.handlers.Notifications.UnarchiveChannel,
			s.scenarioMW(entity.AuthzScenarioNotificationChannelUnarchive))
	}

	// users assignable-picker — the STATIC /users/assignable path. Registered on
	// its own group (not the shared /users group) and, like all auth /users
	// statics, before the auth /users/:id param routes in authProtectedV1Group so
	// the static segment is not shadowed.
	{
		usersAPI := gr.Group("/users", requireToken)
		// Gated on maintenance.create, not .read: the picker only feeds the
		// maintenance form, and its has_messenger_tag flag would otherwise let any
		// guest enumerate the roster and learn who has a messenger connected.
		usersAPI.Add(http.MethodGet, "/assignable", s.handlers.UserPicker.ListAssignableUsers,
			s.scenarioMW(entity.AuthzScenarioMaintenanceCreate))
	}
}

// authProtectedV1Group registers the auth routes that require an access token:
// session (logout/me/providers), role management, user management, invitation
// admin, licensed-seat state and audit read.
//
// Route ordering: every STATIC /users/... route (list, invite, invitations*) is
// registered before the /users/:id... param routes (block/unblock, tags patch).
// That grouping is a readability convention, not a correctness guard — see the
// note at the /users/:id registration for what the router actually does.
func (s *APIServer) authProtectedV1Group(gr *echo.Group) {
	withAuthorize := gr.Group("",
		middlewares.RequireAccessToken(s.security.TokenVerifier),
	)

	withAuthorize.Add(http.MethodPost, "/logout", s.handlers.Auth.Logout)
	withAuthorize.Add(http.MethodPost, "/logout/all", s.handlers.Auth.LogoutAll)
	withAuthorize.Add(http.MethodGet, "/me", s.handlers.Auth.Me)
	withAuthorize.Add(http.MethodPatch, "/me", s.handlers.Auth.UpdateMe)
	withAuthorize.Add(http.MethodPost, "/me/providers/:provider/connect", s.handlers.Auth.ConnectProvider)
	withAuthorize.Add(http.MethodDelete, "/me/providers/:provider/disconnect", s.handlers.Auth.DisconnectProvider)

	withAuthorize.Add(http.MethodGet, "/roles",
		s.handlers.Roles.AvailableRoles,
		middlewares.RequireScenario(s.security.Authorizer, entity.AuthzScenarioAuthRolesRead),
	)
	withAuthorize.Add(http.MethodPost, "/roles/assign",
		s.handlers.Roles.Assign,
		middlewares.RequireScenario(s.security.Authorizer, entity.AuthzScenarioAuthRolesManage),
	)
	withAuthorize.Add(http.MethodPost, "/roles/revoke",
		s.handlers.Roles.Revoke,
		middlewares.RequireScenario(s.security.Authorizer, entity.AuthzScenarioAuthRolesManage),
	)
	withAuthorize.Add(http.MethodGet, "/user/:id/roles",
		s.handlers.Roles.ListRoles,
		middlewares.RequireScenario(s.security.Authorizer, entity.AuthzScenarioAuthUserRolesRead),
	)

	// STATIC /users/... routes first.
	withAuthorize.Add(http.MethodGet, "/users/list",
		s.handlers.Users.ListUsers,
		middlewares.RequireScenario(s.security.Authorizer, entity.AuthzScenarioAuthUsersRead),
	)
	// Admin invitation management. Read uses the users.read scenario; mutating
	// operations use users.manage (same scenarios as block/unblock).
	withAuthorize.Add(http.MethodPost, "/users/invite",
		s.handlers.Invitations.Invite,
		middlewares.RequireScenario(s.security.Authorizer, entity.AuthzScenarioAuthUsersManage),
	)
	withAuthorize.Add(http.MethodGet, "/users/invitations",
		s.handlers.Invitations.ListInvitations,
		middlewares.RequireScenario(s.security.Authorizer, entity.AuthzScenarioAuthUsersRead),
	)
	withAuthorize.Add(http.MethodPost, "/users/invitations/:id/revoke",
		s.handlers.Invitations.RevokeInvitation,
		middlewares.RequireScenario(s.security.Authorizer, entity.AuthzScenarioAuthUsersManage),
	)
	withAuthorize.Add(http.MethodPost, "/users/invitations/:id/resend",
		s.handlers.Invitations.ResendInvitation,
		middlewares.RequireScenario(s.security.Authorizer, entity.AuthzScenarioAuthUsersManage),
	)

	// PARAM /users/:id/... routes after every static /users/... route.
	withAuthorize.Add(http.MethodPost, "/users/:id/block",
		s.handlers.Users.BlockUser,
		middlewares.RequireScenario(s.security.Authorizer, entity.AuthzScenarioAuthUsersManage),
	)
	withAuthorize.Add(http.MethodPost, "/users/:id/unblock",
		s.handlers.Users.UnblockUser,
		middlewares.RequireScenario(s.security.Authorizer, entity.AuthzScenarioAuthUsersManage),
	)
	// The bare /users/:id path is the most greedy of the lot — it matches any
	// single segment — so it is written last, after every static sibling.
	//
	// Note that the router, not this ordering, is what keeps /users/list intact:
	// Echo v5 prefers a static segment over a param regardless of registration
	// order. The consequence to keep in mind is method-scoped — PATCH on a
	// static /users/... path has no handler of its own and falls through to
	// here, where the segment is rejected as a malformed uuid.
	withAuthorize.Add(http.MethodPatch, "/users/:id",
		s.handlers.Users.UpdateUserTags,
		middlewares.RequireScenario(s.security.Authorizer, entity.AuthzScenarioAuthUsersManage),
	)

	// Licensed-seat state for the users page. Registered here, outside the
	// license block gate, on purpose: under a blocked license an admin must still
	// be able to see what is going on with their org's seats.
	withAuthorize.Add(http.MethodGet, "/license/seats",
		s.handlers.Users.GetSeats,
		middlewares.RequireScenario(s.security.Authorizer, entity.AuthzScenarioAuthUsersRead),
	)

	withAuthorize.Add(http.MethodGet, "/audit/log",
		s.handlers.Audit.AuditLog,
		middlewares.RequireScenario(s.security.Authorizer, entity.AuthzScenarioAuthAuditRead),
	)
}

// uiV1Group registers the read-only UI rendering routes (token-gated).
func (s *APIServer) uiV1Group(gr *echo.Group) {
	gr.Add(http.MethodGet, "/calendar", s.handlers.Calendar.CalendarView,
		s.scenarioMW(entity.AuthzScenarioCalendarRead))
	gr.Add(http.MethodGet, "/maintenances/:id", s.handlers.Calendar.MaintView,
		s.scenarioMW(entity.AuthzScenarioMaintenanceRead))
	// Gated on approve rather than read: a page called "awaiting my approval" is
	// not a page a guest sees empty, it is a page a guest does not have. Plain
	// scenarioMW, no introspection — that is reserved for critical mutations.
	gr.Add(http.MethodGet, "/approvals", s.handlers.Approvals.ListApprovals,
		s.scenarioMW(entity.AuthzScenarioMaintenanceApprove))
}
