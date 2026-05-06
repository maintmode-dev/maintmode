package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/public/audit"

	"github.com/ruko1202/maintmode/internal/app/api/public/roles"

	"github.com/ruko1202/maintmode/internal/config/buildmeta"

	"github.com/ruko1202/maintmode/internal/app/api/public/auth"

	"github.com/ruko1202/maintmode/internal/config/redis"

	"github.com/ruko1202/maintmode/internal/app/api/infra"
	"github.com/ruko1202/maintmode/internal/app/bootstrap"
	"github.com/ruko1202/maintmode/internal/config/pg"
	"github.com/ruko1202/maintmode/internal/server"
	"github.com/ruko1202/maintmode/internal/utils/closer"

	"github.com/ruko1202/maintmode/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGQUIT,
	)
	defer shutdown(ctx)
	defer stop()

	cfg := config.LoadAuthAppConfig()
	meta := config.GetAuthAppBuildMeta()

	logger := config.NewLogger(cfg.Environment, cfg.Logger, meta)
	defer logger.Sync() //nolint:errcheck
	xlog.ReplaceGlobalLogger(logger)
	ctx = xlog.ContextWithLogger(ctx, logger)

	xlog.Info(ctx, "start app",
		xfield.Any("environment", cfg.Environment),
		xfield.Any("meta", meta),
	)

	initExporters(ctx, cfg, meta)

	db, err := pg.NewDBConn(ctx, &cfg.DB)
	if err != nil {
		xlog.Panic(ctx, "failed to open db connection", xfield.Error(err))
	}
	closer.Add(db.Close)

	redisClient, err := redis.NewRedis(ctx, &cfg.Redis)
	if err != nil {
		xlog.Panic(ctx, "failed to init redis client", xfield.Error(err))
	}
	closer.Add(redisClient.Close)

	// Bootstrap application layers
	stores := bootstrap.NewAuthStores(db, redisClient)
	services, err := bootstrap.NewAuthServices(cfg, stores)
	if err != nil {
		xlog.Panic(ctx, "failed to init services", xfield.Error(err))
	}

	// start api server
	{
		s := server.NewAPIAuthServer(
			cfg.APIServer,
			cfg.S2SConfig,
			auth.New(
				services.Auth,
				services.Token,
				cfg.App.FrontendURL,
			),
			roles.New(
				services.User,
			),
			audit.New(services.Audit),
			services.Token,
			services.RBAC,
			server.WithLogger(logger),
		)
		s.BindRouters(cfg.Environment, meta)

		go func() {
			if err := s.Start(ctx); err != nil {
				xlog.Fatal(ctx, "api server failed", xfield.Error(err))
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
		s.BindRouters(cfg.Environment, meta.AppName)

		go func() {
			if err := s.Start(ctx); err != nil {
				xlog.Fatal(ctx, "api server failed", xfield.Error(err))
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

func initExporters(ctx context.Context, cfg *config.AppConfig, meta *buildmeta.AppBuildMeta) {
	xlog.ReplaceTracerName(meta.AppName)

	otelRes, err := config.InitTracerResource(meta)
	if err != nil {
		xlog.Panic(ctx, "failed to initialize OpenTelemetry", xfield.Error(err))
	}

	tracerProvider, err := config.InitTracerProvider(ctx, otelRes, cfg.Tracer)
	if err != nil {
		xlog.Panic(ctx, "failed to initialize OpenTelemetry", xfield.Error(err))
	}
	closer.Add(func() error { return tracerProvider.Shutdown(ctx) })

	meterProvider, err := config.InitMetricExporter(otelRes)
	if err != nil {
		xlog.Panic(ctx, "failed to initialize OpenTelemetry metric exporter", xfield.Error(err))
	}
	closer.Add(func() error { return meterProvider.Shutdown(ctx) })
}
