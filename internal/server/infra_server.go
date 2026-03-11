package server

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	echoSwagger "github.com/swaggo/echo-swagger"

	"github.com/ruko1202/maintmode/internal/app/api/infra"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/config/middlewares"
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
	gr := s.e.Group("")
	gr.Use(middlewares.BaseMiddlewares()...)

	gr.RouteNotFound("/*", echo.NotFoundHandler, middlewares.RequestLoggingMiddleware())
	gr.Add(http.MethodGet, "/liveness", s.apiImpl.Liveness)
	gr.Add(http.MethodGet, "/readiness", s.apiImpl.Readiness)
	gr.Add(http.MethodGet, "/version", s.apiImpl.Version)
	// Endpoint for Prometheus scraping
	// this Echo will run on separate port 8081
	gr.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

	if config.GetAppConfig().IsDevEnvironment() {
		gr.Add(http.MethodGet, "/", s.apiImpl.MainPage)
		gr.Add(http.MethodGet, "/swagger/*", echoSwagger.WrapHandler)
	}
}
