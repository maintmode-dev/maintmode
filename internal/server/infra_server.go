package server

import (
	"net/http"

	"github.com/labstack/echo-contrib/v5/pprof"
	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/ruko1202/maintmode/internal/config/buildmeta"
	"github.com/ruko1202/swaggerui"

	"github.com/ruko1202/maintmode/docs"

	"github.com/ruko1202/maintmode/internal/server/middlewares"

	"github.com/ruko1202/maintmode/internal/app/api/infra"

	"github.com/ruko1202/maintmode/internal/config"
)

type InfraServer struct {
	*server
	apiImpl *infra.Implementation
}

func NewInfraServer(
	cfg config.HTTPServer,
	impl *infra.Implementation,
	opts ...Option,
) *InfraServer {
	return &InfraServer{
		server:  newServer(cfg, opts...),
		apiImpl: impl,
	}
}

func (s *InfraServer) BindRouters(env config.Environment, appName string) {
	s.e.Use(middlewares.BaseMiddlewares()...)

	pprof.Register(s.e, pprof.DefaultPrefix)
	gr := s.e.Group("")

	gr.RouteNotFound("/*", s.notFoundHandler, middlewares.RequestLoggingMiddleware())
	gr.Add(http.MethodGet, "/liveness", s.apiImpl.Liveness)
	gr.Add(http.MethodGet, "/readiness", s.apiImpl.Readiness)
	gr.Add(http.MethodGet, "/version", s.apiImpl.Version)
	gr.Add(http.MethodGet, "/metrics", echo.WrapHandler(promhttp.Handler()))

	if env.IsDev() {
		gr.Add(http.MethodGet, "/", s.apiImpl.MainPage)

		// Serve Swagger UI with both service specs selectable from a dropdown.
		// The handler has its own internal mux rooted at "/", so it is mounted
		// under /swagger with the prefix stripped; open it at /swagger/ (the
		// trailing slash matters — the UI assets use relative paths).
		var specsHandler http.Handler
		switch appName {
		case buildmeta.MaintModeAppName:
			specsHandler = swaggerui.HandlerWithSpecs([]swaggerui.Spec{
				{Name: buildmeta.MaintModeAppName, Content: docs.MaintmodeSpec}},
			)
		case buildmeta.AuthAppName:
			specsHandler = swaggerui.HandlerWithSpecs([]swaggerui.Spec{
				{Name: buildmeta.AuthAppName, Content: docs.AuthSpec},
			})
		default:
			specsHandler = swaggerui.HandlerWithSpecs([]swaggerui.Spec{
				{Name: buildmeta.MaintModeAppName, Content: docs.MaintmodeSpec},
				{Name: buildmeta.AuthAppName, Content: docs.AuthSpec},
			}, swaggerui.WithPrimaryName(buildmeta.MaintModeAppName))
		}

		gr.Add(http.MethodGet, "/swagger/*", echo.WrapHandler(http.StripPrefix("/swagger", specsHandler)))
	}
}
