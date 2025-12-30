package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/ruko1202/xlog"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/app/api"
	"github.com/ruko1202/maintmode/internal/app/healthcheck"
	"github.com/ruko1202/maintmode/internal/config/middlewares"
	"github.com/ruko1202/maintmode/internal/config/pg"
	"github.com/ruko1202/maintmode/internal/server"
	"github.com/ruko1202/maintmode/internal/utils/closer"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/config/buildmeta"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(fmt.Errorf("can't initialize zap logger: %w", err))
	}
	closer.Add(logger.Sync)
	xlog.ReplaceGlobal(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGKILL)
	defer stop()

	ctx = xlog.ContextWithLogger(ctx, logger)
	ctx = xlog.WithFields(ctx,
		zap.String("app", config.AppName),
		zap.String("version", buildmeta.GetAppBuildMeta().Version),
	)

	xlog.Info(ctx, "start app", zap.Any("meta", buildmeta.GetAppBuildMeta()))

	cfg := config.GetAppConfig()

	db, err := pg.NewDBConn(ctx, &cfg.DB)
	if err != nil {
		xlog.Fatal(ctx, "failed to open db", zap.Error(err))
	}
	closer.Add(db.Close)

	// start api server
	{
		s := server.NewServer(cfg.APIServer)
		gr := s.NewGroup("api/v1")

		impl := api.New()
		impl.BindRoute(gr, middlewares.BaseWithLoggingMiddlewares("api/v1"))

		go func() {
			if err := s.Start(ctx); err != nil {
				xlog.Fatal(ctx, "api server failed", zap.Error(err))
			}
		}()
		closer.AddWithName("api server", func() error { return s.Stop(context.Background()) })
	}

	// start status server
	{
		s := server.NewServer(cfg.StatusServer)
		gr := s.NewGroup("")

		impl := healthcheck.NewImplementation(db)
		impl.BindRoute(gr, middlewares.BaseMiddlewares())

		go func() {
			if err := s.Start(ctx); err != nil {
				xlog.Fatal(ctx, "api server failed", zap.Error(err))
			}
		}()

		closer.AddWithName("status server", func() error { return s.Stop(context.Background()) })
	}

	<-ctx.Done()
	xlog.Info(ctx, "graceful shutdown...")
	closer.CloseAll(context.Background())
	xlog.Info(ctx, "app is gracefully shutdown")
}
