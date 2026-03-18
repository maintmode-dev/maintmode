package server

import (
	"net/http"

	"github.com/labstack/echo-contrib/pprof"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	echoSwagger "github.com/swaggo/echo-swagger"

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
) *InfraServer {
	return &InfraServer{
		server:  newServer(cfg),
		apiImpl: impl,
	}
}

func (s *InfraServer) BindRouters() {
	s.e.Use(middlewares.BaseMiddlewares()...)

	pprof.Register(s.e, pprof.DefaultPrefix)
	gr := s.e.Group("")

	gr.RouteNotFound("/*", echo.NotFoundHandler, middlewares.RequestLoggingMiddleware())
	gr.Add(http.MethodGet, "/liveness", s.apiImpl.Liveness)
	gr.Add(http.MethodGet, "/readiness", s.apiImpl.Readiness)
	gr.Add(http.MethodGet, "/version", s.apiImpl.Version)
	gr.Add(http.MethodGet, "/metrics", echo.WrapHandler(promhttp.Handler()))

	if config.GetAppConfig().Environment.IsDev() {
		gr.Add(http.MethodGet, "/", s.apiImpl.MainPage)
		gr.Add(http.MethodGet, "/swagger/*", echoSwagger.WrapHandler)
	}
}
