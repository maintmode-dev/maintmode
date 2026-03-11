package server

import (
	"net/http"
	"time"

	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"go.opentelemetry.io/otel/trace"

	"github.com/ruko1202/maintmode/internal/config/buildmeta"

	apimaint "github.com/ruko1202/maintmode/internal/app/api/public/maint"
	resourcesapi "github.com/ruko1202/maintmode/internal/app/api/public/resources"
	uicalendar "github.com/ruko1202/maintmode/internal/app/api/ui/calendar"

	"github.com/ruko1202/maintmode/internal/config"

	_ "github.com/ruko1202/maintmode/docs" // swagger docs

	"github.com/ruko1202/maintmode/internal/config/middlewares"
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
) *APIServer {
	return &APIServer{
		server:        newServer(cfg),
		maintImpl:     maintImpl,
		resourcesImpl: resourcesImpl,
		calendarImpl:  calendarImpl,
	}
}

func (s *APIServer) BindRouters() {
	mwares := append(
		middlewares.BaseMiddlewares(),
		middlewares.RequestLoggingMiddleware(),
		middleware.ContextTimeout(60*time.Second),
		echoprometheus.NewMiddleware("maintmode"), // middleware to gather metrics. route to serve metrics on infra port
	)

	if config.GetAppConfig().IsDevEnvironment() {
		mwares = append(mwares,
			middlewares.ReqReqsDumpLoggingMiddleware(),
		)
	}

	rootGr := s.e.Group("")
	rootGr.Use(mwares...)
	rootGr.RouteNotFound("/*", echo.NotFoundHandler, middlewares.RequestLoggingMiddleware())
	// Middleware for parsing traces from HTTP
	rootGr.Use(otelecho.Middleware(buildmeta.GetAppBuildMeta().AppName))
	// Middleware for injecting TraceID into X-Request-ID response header
	rootGr.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()
			spanCtx := trace.SpanContextFromContext(ctx)

			// If trace was successfully created/received
			if spanCtx.HasTraceID() {
				traceID := spanCtx.TraceID().String()

				// Return to client in response header
				c.Response().Header().Set(echo.HeaderXRequestID, traceID)

				// Store in Echo context (so Echo's standard loggers can pick it up)
				c.Set(echo.HeaderXRequestID, traceID)
			}
			return next(c)
		}
	})

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
