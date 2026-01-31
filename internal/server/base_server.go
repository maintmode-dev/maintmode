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

type server struct {
	cfg config.HTTPServer
	e   *echo.Echo
}

// newServer creates a new HTTP server with the provided configuration.
func newServer(cfg config.HTTPServer) *server {
	return &server{
		cfg: cfg,
		e:   echo.New(),
	}
}

// Start begins listening and blocks until server stops.
// Returns error only if startup fails, otherwise blocks until shutdown.
func (s *server) Start(ctx context.Context) error {
	xlog.Infof(ctx, "starting http server '%s' on %s", s.cfg.Name, s.cfg.BuildHostPort())

	err := s.e.Start(s.cfg.BuildHostPort())
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("failed to start http server '%s': %w", s.cfg.Name, err)
	}
	return nil
}

// Stop gracefully shuts down the server with context-based timeout control.
func (s *server) Stop(ctx context.Context) error {
	xlog.Infof(ctx, "shutdown http server: '%s'", s.cfg.Name)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := s.e.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown http server '%s' failed: %w", s.cfg.Name, err)
	}
	xlog.Infof(ctx, "http server is shutdown: '%s'", s.cfg.Name)

	return nil
}

func (s *server) BindRouters() {
	xlog.Panic(context.Background(), "implement me")
}
