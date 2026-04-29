package server

import (
	"net/http"

	"github.com/labstack/echo-contrib/v5/pprof"
	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	echoSwagger "github.com/swaggo/echo-swagger/v2"

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
		if env.IsDev() && !env.IsLocal() {
			docs.SwaggerInfo.Host = "localhost:9000"
			docs.SwaggerInfo.BasePath = "/" + appName
		}

		gr.Add(http.MethodGet, "/swagger/*", echoSwagger.WrapHandler)
	}
}
