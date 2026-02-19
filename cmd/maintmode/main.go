package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/ruko1202/xlog"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/app/api/infra"
	apimaint "github.com/ruko1202/maintmode/internal/app/api/public/maint"
	uicalendar "github.com/ruko1202/maintmode/internal/app/api/ui/calendar"

	"github.com/ruko1202/maintmode/internal/services/calendar"

	conflictsSvr "github.com/ruko1202/maintmode/internal/services/conflicts"
	maintSrv "github.com/ruko1202/maintmode/internal/services/maint"
	conflictsnapshots "github.com/ruko1202/maintmode/internal/storages/conflict_snapshots"
	"github.com/ruko1202/maintmode/internal/storages/conflicts"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"

	"github.com/ruko1202/maintmode/internal/config/pg"
	"github.com/ruko1202/maintmode/internal/server"
	"github.com/ruko1202/maintmode/internal/utils/closer"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/config/buildmeta"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGKILL,
	)
	defer stop()
	defer closer.CloseAll(context.WithoutCancel(ctx))

	cfg := config.GetAppConfig()

	logger := config.NewLogger(cfg.Environment)
	closer.Add(logger.Sync)
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

	// start api server
	{
		s := server.NewAPIServer(
			cfg.APIServer,
			apimaint.New(
				maintSrv.NewService(
					dbtx.NewTxManager(db),
					maintenances.NewStore(db),
					resources.NewStore(db),
					conflictsnapshots.NewStore(db),
					conflictsSvr.NewService(conflicts.NewStore(db)),
				),
			),
			uicalendar.New(calendar.NewService(
				maintenances.NewStore(db),
				resources.NewStore(db),
				conflictsSvr.NewService(conflicts.NewStore(db))),
			),
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
			infra.NewImplementation(db),
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
	xlog.Info(ctx, "graceful shutdown...")
	closer.CloseAll(context.WithoutCancel(ctx))
	xlog.Info(ctx, "app is gracefully shutdown")
}
