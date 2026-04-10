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
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

type Option func(*server)

func WithLogger(l xlog.Logger) Option {
	return func(s *server) {
		s.e.Logger = xecho.NewLogAdapter(l)
	}
}

type server struct {
	cfg config.HTTPServer
	e   *echo.Echo
}

// newServer creates a new HTTP server with the provided configuration.
func newServer(cfg config.HTTPServer, opts ...Option) *server {
	s := &server{
		cfg: cfg,
		e:   echo.New(),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Start begins listening and blocks until server stops.
// Returns error only if startup fails, otherwise blocks until shutdown.
// The provided context is used to stop blocking when canceled.
// Actual shutdown is performed by the Stop method.
func (s *server) Start(ctx context.Context) error {
	xlog.Infof(ctx, "starting http server '%s' on %s", s.cfg.Name, s.cfg.BuildHostPort())

	// Start server in a goroutine to allow context-based unblocking
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.e.Start(s.cfg.BuildHostPort())
	}()

	// Wait for either server startup error or context cancellation
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("failed to start http server '%s': %w", s.cfg.Name, err)
		}
		return nil
	case <-ctx.Done():
		// Context canceled, unblock the Start call
		// Actual shutdown will be performed by Stop method via closer
		xlog.Infof(ctx, "http server '%s' start unblocked due to context cancellation", s.cfg.Name)
		return nil
	}
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
