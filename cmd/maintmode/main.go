package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/ruko1202/xlog"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/app/api/infra"
	apimaint "github.com/ruko1202/maintmode/internal/app/api/public/maint"
	resourcesapi "github.com/ruko1202/maintmode/internal/app/api/public/resources"
	uicalendar "github.com/ruko1202/maintmode/internal/app/api/ui/calendar"
	"github.com/ruko1202/maintmode/internal/app/bootstrap"
	"github.com/ruko1202/maintmode/internal/server"

	"github.com/ruko1202/maintmode/internal/config/pg"
	"github.com/ruko1202/maintmode/internal/utils/closer"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/config/buildmeta"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGQUIT,
	)
	defer shutdown(ctx)
	defer stop()

	cfg := config.GetAppConfig()

	logger := config.NewLogger(cfg.Environment)
	defer logger.Sync() //nolint:errcheck
	xlog.ReplaceGlobal(logger)

	ctx = xlog.ContextWithLogger(ctx, logger)
	ctx = xlog.WithFields(ctx,
		zap.String("app", config.AppName),
		zap.String("version", buildmeta.GetAppBuildMeta().Version),
	)

	xlog.Info(ctx, "start app", zap.Any("meta", buildmeta.GetAppBuildMeta()))

	db, err := pg.NewDBConn(ctx, &cfg.DB)
	if err != nil {
		xlog.Panic(ctx, "failed to open db connection", zap.Error(err))
	}
	closer.Add(db.Close)

	// Bootstrap application layers
	stores := bootstrap.NewStores(db)
	services := bootstrap.NewServices(stores)

	// start api server
	{
		s := server.NewAPIServer(
			cfg.APIServer,
			apimaint.New(services.Maint),
			resourcesapi.New(services.Resources),
			uicalendar.New(services.Calendar),
		)
		s.BindRouters()

		go func() {
			if err := s.Start(ctx); err != nil {
				xlog.Fatal(ctx, "api server failed", zap.Error(err))
			}
		}()
		closer.AddWithName("api server", func() error { return s.Stop(context.Background()) })
	}

	// start infra server: healthcheck, metrics, etc.
	{
		s := server.NewInfraServer(
			cfg.InfraServer,
			infra.New(db),
		)
		s.BindRouters()

		go func() {
			if err := s.Start(ctx); err != nil {
				xlog.Fatal(ctx, "api server failed", zap.Error(err))
			}
		}()

		closer.AddWithName("status server", func() error { return s.Stop(context.Background()) })
	}

	<-ctx.Done()
}

func shutdown(ctx context.Context) {
	xlog.Info(ctx, "graceful shutdown...")
	closer.CloseAll(context.WithoutCancel(ctx))
	xlog.Info(ctx, "app is gracefully shutdown")
}
