package server

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/ruko1202/maintmode/internal/config/buildmeta"

	_ "github.com/ruko1202/maintmode/docs" // swagger docs
	apimaint "github.com/ruko1202/maintmode/internal/app/api/public/maint"
	resourcesapi "github.com/ruko1202/maintmode/internal/app/api/public/resources"
	uicalendar "github.com/ruko1202/maintmode/internal/app/api/ui/calendar"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/server/middlewares"
)

type APIServer struct {
	*server
	maintImpl     *apimaint.Implementation
	resourcesImpl *resourcesapi.Implementation
	calendarImpl  *uicalendar.Implementation
	tokenVerifier middlewares.TokenVerifier
	authorizer    middlewares.Authorizer
}

func NewAPIServer(
	cfg config.HTTPServer,
	maintImpl *apimaint.Implementation,
	resourcesImpl *resourcesapi.Implementation,
	calendarImpl *uicalendar.Implementation,
	tokenVerifier middlewares.TokenVerifier,
	authorizer middlewares.Authorizer,
	opts ...Option,
) *APIServer {
	return &APIServer{
		server:        newServer(cfg, opts...),
		maintImpl:     maintImpl,
		resourcesImpl: resourcesImpl,
		calendarImpl:  calendarImpl,
		tokenVerifier: tokenVerifier,
		authorizer:    authorizer,
	}
}

func (s *APIServer) BindRouters(env config.Environment, meta *buildmeta.AppBuildMeta) {
	rootGr := s.e.Group("")
	rootGr.Use(middlewares.BaseAPIMiddlewares(env, meta)...)
	rootGr.RouteNotFound("/*", s.notFoundHandler, middlewares.RequestLoggingMiddleware())

	s.apiV1Group(rootGr.Group("/api/v1",
		middlewares.RequireAccessToken(s.tokenVerifier),
	))
	s.uiV1Group(rootGr.Group("/ui/v1",
		middlewares.RequireAccessToken(s.tokenVerifier),
	))
}

// api V1 API group
func (s *APIServer) apiV1Group(gr *echo.Group) {
	// maint API group
	{
		maintAPI := gr.Group("/maintenances")
		maintAPI.Add(http.MethodPost, "/create",
			s.maintImpl.CreateDraftMaint,
			middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioMaintenanceCreate),
		)
		maintAPI.Add(http.MethodPost, "/:id/edit",
			s.maintImpl.UpdateDraftMaint,
			middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioMaintenanceEdit),
		)
		maintAPI.Add(http.MethodPost, "/:id/start",
			s.maintImpl.StartMaint,
			middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioMaintenanceStart),
		)
		maintAPI.Add(http.MethodPost, "/:id/cancel",
			s.maintImpl.CancelMaint,
			middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioMaintenanceCancel),
		)
		maintAPI.Add(http.MethodPost, "/:id/complete",
			s.maintImpl.CompleteMaint,
			middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioMaintenanceComplete),
		)
		maintAPI.Add(http.MethodPost, "/:id/approve",
			s.maintImpl.ApproveMaint,
			middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioMaintenanceApprove),
		)
		maintAPI.Add(http.MethodPost, "/:id/steps/:step_id/start",
			s.maintImpl.StartStep,
			middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioMaintenanceStepStart),
		)
		maintAPI.Add(http.MethodPost, "/:id/steps/:step_id/complete",
			s.maintImpl.CompleteStep,
			middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioMaintenanceStepComplete),
		)
		maintAPI.Add(http.MethodPost, "/:id/steps/:step_id/cancel",
			s.maintImpl.CancelStep,
			middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioMaintenanceStepCancel),
		)
		maintAPI.Add(http.MethodGet, "/:id",
			s.maintImpl.GetMaint,
			middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioMaintenanceRead),
		)
	}

	// resources API group
	{
		resourcesAPI := gr.Group("/resources")
		resourcesAPI.Add(http.MethodGet, "",
			s.resourcesImpl.SearchResources,
			middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioResourceRead),
		)
	}

	// resource API group
	{
		resourceAPI := gr.Group("/resource")
		resourceAPI.Add(http.MethodGet, "/:id/types",
			s.resourcesImpl.GetResourceTypes,
			middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioResourceRead),
		)
		resourceAPI.Add(http.MethodPost, "/create",
			s.resourcesImpl.CreateResource,
			middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioResourceCreate),
		)
	}
}

// ui V1 API group
func (s *APIServer) uiV1Group(gr *echo.Group) {
	gr.Add(http.MethodGet, "/calendar",
		s.calendarImpl.CalendarView,
		middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioCalendarRead),
	)
	gr.Add(http.MethodGet, "/maintenances/:id",
		s.calendarImpl.MaintView,
		middlewares.RequireScenario(s.authorizer, entity.AuthzScenarioMaintenanceRead),
	)
}
