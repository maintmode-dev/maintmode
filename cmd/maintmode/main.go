package main

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	redislib "github.com/redis/go-redis/v9"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/config/buildmeta"

	"github.com/ruko1202/maintmode/internal/app/api/infra"
	apiaudit "github.com/ruko1202/maintmode/internal/app/api/public/audit"
	apiauth "github.com/ruko1202/maintmode/internal/app/api/public/auth"
	apiinvitations "github.com/ruko1202/maintmode/internal/app/api/public/invitations"
	apimaint "github.com/ruko1202/maintmode/internal/app/api/public/maint"
	apinotifications "github.com/ruko1202/maintmode/internal/app/api/public/notifytargets"
	resourcesapi "github.com/ruko1202/maintmode/internal/app/api/public/resources"
	apiroles "github.com/ruko1202/maintmode/internal/app/api/public/roles"
	userpickerapi "github.com/ruko1202/maintmode/internal/app/api/public/userpicker"
	apiusers "github.com/ruko1202/maintmode/internal/app/api/public/users"
	uicalendar "github.com/ruko1202/maintmode/internal/app/api/ui/calendar"
	"github.com/ruko1202/maintmode/internal/app/bootstrap"
	"github.com/ruko1202/maintmode/internal/config/pg"
	"github.com/ruko1202/maintmode/internal/config/redis"
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

	cfg := config.LoadAppConfig()
	meta := config.GetAppBuildMeta()

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
	stores, err := bootstrap.NewStores(db, redisClient)
	if err != nil {
		xlog.Panic(ctx, "failed to init storages", xfield.Error(err))
	}
	gateways, err := bootstrap.NewGateways(cfg)
	if err != nil {
		xlog.Panic(ctx, "failed to init gateways", xfield.Error(err))
	}
	services, err := bootstrap.NewServices(ctx, cfg, stores, gateways)
	if err != nil {
		xlog.Panic(ctx, "failed to init services", xfield.Error(err))
	}

	// start async task processor
	{
		taskProcessors, err := bootstrap.NewTaskProcessors(cfg.TaskProcessor, stores, services, gateways)
		if err != nil {
			xlog.Panic(ctx, "failed to init task processors", xfield.Error(err))
		}
		closer.Add(closer.NoErrCloseFunc(taskProcessors.Stop))
		go func() {
			if err := taskProcessors.Run(ctx); err != nil {
				xlog.Fatal(ctx, "messaging goque exited with error", xfield.Error(err))
			}
		}()
	}

	startAPIServer(ctx, cfg, meta, services, redisClient, logger)

	// Owns the drain signal: main flips it on shutdown, the readiness handler
	// only reads it. Keeps process-lifecycle state out of the HTTP layer.
	drainer := lifecycle.NewDrainer()

	startInfraServer(ctx, cfg, db, drainer, logger)

	<-ctx.Done()
	shutdown(context.WithoutCancel(ctx), drainer, cfg.Shutdown.DrainTimeoutOrDefault())
}

// startAPIServer wires the per-domain handlers, starts the public API server in
// a goroutine and registers its graceful stop with the closer.
func startAPIServer(
	ctx context.Context,
	cfg *config.AppConfig,
	meta *buildmeta.AppBuildMeta,
	services *bootstrap.Services,
	redisClient *redislib.Client,
	logger xlog.Logger,
) {
	s := server.NewAPIServer(
		cfg.APIServer,
		server.APIServerHandlers{
			Maint:         apimaint.New(services.Maint, services.UserSummary),
			Resources:     resourcesapi.New(services.Resources, services.UserSummary),
			Calendar:      uicalendar.New(services.Calendar, services.RBAC, services.UserSummary),
			Notifications: apinotifications.New(services.NotifyTargets, services.UserSummary),
			UserPicker:    userpickerapi.New(services.UserPicker),

			Auth: apiauth.New(
				services.Auth,
				services.Token,
				services.User,
				services.StateCodec,
				cfg.App.FrontendURL,
			),
			Roles:       apiroles.New(services.User),
			Users:       apiusers.New(services.User),
			Invitations: apiinvitations.New(services.Invitation),
			Audit:       apiaudit.New(services.Audit),
		},
		server.APIServerSecurity{
			TokenVerifier: services.JWTVerifier,
			TokenChecker:  services.TokenChecker,
			Authorizer:    services.RBAC,
		},
		redisClient,
		server.WithLogger(logger),
	)
	s.BindRouters(cfg.Environment, meta)

	go func() {
		if err := s.Start(ctx); err != nil {
			xlog.Fatal(ctx, "api server failed", xfield.Error(err))
		}
	}()
	closer.AddWithName("api server", closer.NoCtxCloseFunc(func() error {
		return s.Stop(context.Background())
	}))
}

// startInfraServer starts the infra server (healthcheck, metrics) in a
// goroutine and registers its graceful stop with the closer.
func startInfraServer(
	ctx context.Context,
	cfg *config.AppConfig,
	db *sqlx.DB,
	drainer *lifecycle.Drainer,
	logger xlog.Logger,
) {
	s := server.NewInfraServer(
		cfg.InfraServer,
		infra.New(db, drainer),
		server.WithLogger(logger),
	)
	s.BindRouters(cfg.Environment)

	go func() {
		if err := s.Start(ctx); err != nil {
			xlog.Fatal(ctx, "infra server failed", xfield.Error(err))
		}
	}()

	closer.AddWithName("status server", closer.NoCtxCloseFunc(func() error {
		return s.Stop(context.Background())
	}))
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
