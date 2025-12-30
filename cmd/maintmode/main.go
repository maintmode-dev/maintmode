package main

import (
	"context"

	"github.com/ruko1202/xlog"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/config/buildmeta"
)

func main() {
	ctx := context.Background()
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	ctx = xlog.ContextWithLogger(ctx, logger)
	ctx = xlog.WithFields(ctx, zap.String("app", config.AppName))

	xlog.Info(ctx, "start app", zap.Any("meta", buildmeta.GetAppBuildMeta()))
	xlog.Info(ctx, "app started")
	xlog.Info(ctx, "start graceful shutdown")
	xlog.Info(ctx, "app is gracefully shutdown")
}
