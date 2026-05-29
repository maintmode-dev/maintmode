package server

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/ruko1202/maintmode/internal/config/buildmeta"

	_ "github.com/ruko1202/maintmode/docs" // swagger docs
	apimaint "github.com/ruko1202/maintmode/internal/app/api/public/maint"
	apinotifications "github.com/ruko1202/maintmode/internal/app/api/public/notifytargets"
	resourcesapi "github.com/ruko1202/maintmode/internal/app/api/public/resources"
	uicalendar "github.com/ruko1202/maintmode/internal/app/api/ui/calendar"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/server/middlewares"
)

// APIServerHandlers holds the per-domain Echo handler implementations served
// by the API server. Adding a new domain means adding a field here, not a new
// constructor argument.
type APIServerHandlers struct {
	Maint         *apimaint.Implementation
	Resources     *resourcesapi.Implementation
	Calendar      *uicalendar.Implementation
	Notifications *apinotifications.Implementation
}

// APIServerSecurity holds the security primitives wired into middleware
// chains: local JWT verification, auth-gateway introspect for active-token
// checks on critical mutations, and RBAC scenario authorization.
type APIServerSecurity struct {
	TokenVerifier middlewares.TokenVerifier
	Introspector  middlewares.ActiveTokenIntrospector
	Authorizer    middlewares.Authorizer
}

type APIServer struct {
	*server
	handlers APIServerHandlers
	security APIServerSecurity
}

func NewAPIServer(
	cfg config.HTTPServer,
	handlers APIServerHandlers,
	security APIServerSecurity,
	opts ...Option,
) *APIServer {
	return &APIServer{
		server:   newServer(cfg, opts...),
		handlers: handlers,
		security: security,
	}
}

func (s *APIServer) BindRouters(env config.Environment, meta *buildmeta.AppBuildMeta) {
	rootGr := s.e.Group("")
	rootGr.Use(middlewares.BaseAPIMiddlewares(env, meta)...)
	rootGr.RouteNotFound("/*", s.notFoundHandler, middlewares.RequestLoggingMiddleware())

	s.apiV1Group(rootGr.Group("/api/v1",
		middlewares.RequireAccessToken(s.security.TokenVerifier),
	))
	s.uiV1Group(rootGr.Group("/ui/v1",
		middlewares.RequireAccessToken(s.security.TokenVerifier),
	))
}

func (s *APIServer) scenarioMW(scenario entity.AuthzScenario) echo.MiddlewareFunc {
	return middlewares.RequireScenario(s.security.Authorizer, scenario)
}

func (s *APIServer) scenarioWithIntrospectMW(scenario entity.AuthzScenario) []echo.MiddlewareFunc {
	return []echo.MiddlewareFunc{
		middlewares.RequireActiveToken(s.security.Introspector),
		s.scenarioMW(scenario),
	}
}

// api V1 API group
func (s *APIServer) apiV1Group(gr *echo.Group) {
	// maint API group
	{
		maintAPI := gr.Group("/maintenances")
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
		resourcesAPI := gr.Group("/resources")
		resourcesAPI.Add(http.MethodGet, "", s.handlers.Resources.SearchResources,
			s.scenarioMW(entity.AuthzScenarioResourceRead))
	}

	// resource API group
	{
		resourceAPI := gr.Group("/resource")
		resourceAPI.Add(http.MethodPost, "/create", s.handlers.Resources.CreateResource,
			s.scenarioMW(entity.AuthzScenarioResourceCreate))
	}

	// notifications API group — channel catalog powering the admin
	// picker. Read-scoped on the maintenance scenario since the picker
	// is part of the maintenance edit flow.
	{
		notifAPI := gr.Group("/notifications")
		notifAPI.Add(http.MethodGet, "/channels", s.handlers.Notifications.GetChannels,
			s.scenarioMW(entity.AuthzScenarioMaintenanceRead))
	}
}

// ui V1 API group
func (s *APIServer) uiV1Group(gr *echo.Group) {
	gr.Add(http.MethodGet, "/calendar", s.handlers.Calendar.CalendarView,
		s.scenarioMW(entity.AuthzScenarioCalendarRead))
	gr.Add(http.MethodGet, "/maintenances/:id", s.handlers.Calendar.MaintView,
		s.scenarioMW(entity.AuthzScenarioMaintenanceRead))
}
