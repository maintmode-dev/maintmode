package server

import (
	"net/http"
	"time"

	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	apimaint "github.com/ruko1202/maintmode/internal/app/api/public/maint"
	uicalendar "github.com/ruko1202/maintmode/internal/app/api/ui/calendar"

	"github.com/ruko1202/maintmode/internal/config"

	_ "github.com/ruko1202/maintmode/docs" // swagger docs

	"github.com/ruko1202/maintmode/internal/config/middlewares"
)

type APIServer struct {
	*server
	maintImpl    *apimaint.Implementation
	calendarImpl *uicalendar.Implementation
}

func NewAPIServer(
	cfg config.HTTPServer,
	maintImpl *apimaint.Implementation,
	calendarImpl *uicalendar.Implementation,
) *APIServer {
	return &APIServer{
		server:       newServer(cfg),
		maintImpl:    maintImpl,
		calendarImpl: calendarImpl,
	}
}

func (s *APIServer) BindRouters() {
	rootGr := s.e.Group("")
	rootGr.Use(append(
		middlewares.BaseMiddlewares(),
		middlewares.ReqReqsDumpLoggingMiddleware(),
		middlewares.RequestLoggingMiddleware(),
		middleware.ContextTimeout(60*time.Second),
		echoprometheus.NewMiddleware("maintmode"), // middleware to gather metrics. route to serve metrics on infra port
	)...)
	rootGr.RouteNotFound("/*", echo.NotFoundHandler, middlewares.RequestLoggingMiddleware())

	apiGr := rootGr.Group("/api/v1")
	apiGr.Add(http.MethodPost, "/maintenances/create", s.maintImpl.CreateDraftMaint)
	apiGr.Add(http.MethodPost, "/maintenances/:id/edit", s.maintImpl.UpdateDraftMaint)
	apiGr.Add(http.MethodPost, "/maintenances/:id/start", s.maintImpl.StartMaint)
	apiGr.Add(http.MethodPost, "/maintenances/:id/cancel", s.maintImpl.CancelMaint)
	apiGr.Add(http.MethodPost, "/maintenances/:id/complete", s.maintImpl.CompleteMaint)
	apiGr.Add(http.MethodPost, "/maintenances/:id/approve", s.maintImpl.ApproveMaint)
	apiGr.Add(http.MethodGet, "/maintenances/:id", s.maintImpl.GetMaint)

	uiGr := rootGr.Group("/ui/v1")
	uiGr.Add(http.MethodGet, "/calendar", s.calendarImpl.CalendarView)
	uiGr.Add(http.MethodGet, "/maintenances/:id", s.calendarImpl.MaintView)
}
