// Package server provides HTTP server management with Echo framework.
// It handles server lifecycle including start, stop, and graceful shutdown.
package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/config"
)

// RouterBinder defines the interface for components that can bind routes to Echo groups.
type RouterBinder interface {
	BindRoute(gr *echo.Group, middlewares []echo.MiddlewareFunc)
}

// Server wraps an Echo instance with configuration and lifecycle management.
type Server struct {
	cfg config.HTTPServer
	e   *echo.Echo
}

// NewServer creates a new HTTP server with the provided configuration.
func NewServer(cfg config.HTTPServer) *Server {
	return &Server{
		cfg: cfg,
		e:   echo.New(),
	}
}

// NewGroup creates a new route group with the given prefix.
func (s *Server) NewGroup(prefix string) *echo.Group {
	return s.e.Group(prefix)
}

// Start begins listening and blocks until server stops.
// Returns error only if startup fails, otherwise blocks until shutdown.
func (s *Server) Start(ctx context.Context) error {
	xlog.Infof(ctx, "starting http server '%s' on %s", s.cfg.Name, s.cfg.BuildHostPort())

	err := s.e.Start(s.cfg.BuildHostPort())
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("failed to start http server '%s': %w", s.cfg.Name, err)
	}
	return nil
}

// Stop gracefully shuts down the server with context-based timeout control.
func (s *Server) Stop(ctx context.Context) error {
	xlog.Infof(ctx, "shutdown http server: '%s'", s.cfg.Name)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := s.e.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown http server '%s' failed: %w", s.cfg.Name, err)
	}
	xlog.Infof(ctx, "http server is shutdown: '%s'", s.cfg.Name)

	return nil
}
