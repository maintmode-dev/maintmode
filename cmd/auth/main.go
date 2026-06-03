package main

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/public/audit"

	"github.com/ruko1202/maintmode/internal/app/api/public/roles"
	"github.com/ruko1202/maintmode/internal/app/api/public/users"

	"github.com/ruko1202/maintmode/internal/config/buildmeta"

	"github.com/ruko1202/maintmode/internal/app/api/public/auth"

	"github.com/ruko1202/maintmode/internal/config/redis"

	"github.com/ruko1202/maintmode/internal/app/api/infra"
	"github.com/ruko1202/maintmode/internal/app/bootstrap"
	"github.com/ruko1202/maintmode/internal/config/pg"
	"github.com/ruko1202/maintmode/internal/lifecycle"
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
	services, err := bootstrap.NewAuthServices(ctx, cfg, stores)
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
				services.User,
				services.StateCodec,
				cfg.App.FrontendURL,
			),
			roles.New(
				services.User,
			),
			users.New(
				services.User,
			),
			audit.New(services.Audit),
			services.Token,
			services.RBAC,
			redisClient,
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

	// Owns the drain signal: main flips it on shutdown, the readiness handler
	// only reads it. Keeps process-lifecycle state out of the HTTP layer.
	drainer := lifecycle.NewDrainer()

	// start infra server: healthcheck, metrics, etc.
	{
		s := server.NewInfraServer(
			cfg.InfraServer,
			infra.New(db, drainer),
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
	shutdown(context.WithoutCancel(ctx), drainer, cfg.Shutdown.DrainTimeoutOrDefault())
}

// shutdown drains the replica before tearing it down: it first flips
// Readiness to 503 and waits drainTimeout so the reverse proxy ejects this
// instance from its pool, then closes all registered resources. This ordering
// is what keeps rolling deploys free of 5xx — the load balancer stops sending
// new requests here while the HTTP server is still up to finish in-flight ones.
//
// Invariant: drainTimeout must exceed (Caddy health_interval + the slowest
// expected in-flight request). closer.CloseAll closes the DB before the HTTP
// server's graceful drain completes, so any request still running when
// CloseAll fires can hit a closed DB → 5xx. The drain wait exists precisely
// so no such request is in flight by then; do not shrink it below that bound.
func shutdown(ctx context.Context, drainer *lifecycle.Drainer, drainTimeout time.Duration) {
	xlog.Info(ctx, "graceful shutdown: draining...", xfield.Any("drain_timeout", drainTimeout))
	drainer.StartDraining()

	// Hold for the full drain window: ctx here is non-cancelable
	// (context.WithoutCancel), so this is an unconditional wait — the signal
	// that triggered shutdown has already fired and must not cut the drain
	// short, or the proxy may still be routing to us when we tear down.
	timer := time.NewTimer(drainTimeout)
	defer timer.Stop()
	<-timer.C

	closer.CloseAll(ctx)
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
