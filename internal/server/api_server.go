package server

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/ruko1202/maintmode/internal/config/buildmeta"

	apimaint "github.com/ruko1202/maintmode/internal/app/api/public/maint"
	resourcesapi "github.com/ruko1202/maintmode/internal/app/api/public/resources"
	uicalendar "github.com/ruko1202/maintmode/internal/app/api/ui/calendar"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/server/middlewares"

	_ "github.com/ruko1202/maintmode/docs" // swagger docs
)

type APIServer struct {
	*server
	maintImpl     *apimaint.Implementation
	resourcesImpl *resourcesapi.Implementation
	calendarImpl  *uicalendar.Implementation
}

func NewAPIServer(
	cfg config.HTTPServer,
	maintImpl *apimaint.Implementation,
	resourcesImpl *resourcesapi.Implementation,
	calendarImpl *uicalendar.Implementation,
	opts ...Option,
) *APIServer {
	return &APIServer{
		server:        newServer(cfg, opts...),
		maintImpl:     maintImpl,
		resourcesImpl: resourcesImpl,
		calendarImpl:  calendarImpl,
	}
}

func (s *APIServer) BindRouters(env config.Environment, meta *buildmeta.AppBuildMeta) {
	rootGr := s.e.Group("")
	rootGr.Use(middlewares.BaseAPIMiddlewares(env, meta)...)
	rootGr.RouteNotFound("/*", s.notFoundHandler, middlewares.RequestLoggingMiddleware())

	s.apiV1Group(rootGr.Group("/api/v1"))
	s.uiV1Group(rootGr.Group("/ui/v1"))
}

// api V1 API group
func (s *APIServer) apiV1Group(gr *echo.Group) {
	// maint API group
	{
		maintAPI := gr.Group("/maintenances")
		maintAPI.Add(http.MethodPost, "/create", s.maintImpl.CreateDraftMaint)
		maintAPI.Add(http.MethodPost, "/:id/edit", s.maintImpl.UpdateDraftMaint)
		maintAPI.Add(http.MethodPost, "/:id/start", s.maintImpl.StartMaint)
		maintAPI.Add(http.MethodPost, "/:id/cancel", s.maintImpl.CancelMaint)
		maintAPI.Add(http.MethodPost, "/:id/complete", s.maintImpl.CompleteMaint)
		maintAPI.Add(http.MethodPost, "/:id/approve", s.maintImpl.ApproveMaint)
		maintAPI.Add(http.MethodGet, "/:id", s.maintImpl.GetMaint)
	}

	// resources API group
	{
		resourcesAPI := gr.Group("/resources")
		resourcesAPI.Add(http.MethodGet, "", s.resourcesImpl.SearchResources)
	}

	// resource API group
	{
		resourceAPI := gr.Group("/resource")
		resourceAPI.Add(http.MethodGet, "/:id/types", s.resourcesImpl.GetResourceTypes)
		resourceAPI.Add(http.MethodPost, "/create", s.resourcesImpl.CreateResource)
	}
}

// ui V1 API group
func (s *APIServer) uiV1Group(gr *echo.Group) {
	gr.Add(http.MethodGet, "/calendar", s.calendarImpl.CalendarView)
	gr.Add(http.MethodGet, "/maintenances/:id", s.calendarImpl.MaintView)
}
