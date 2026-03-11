package config

import (
	"fmt"
	"log"

	"github.com/ruko1202/xlog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/ruko1202/maintmode/internal/config/buildmeta"
)

func NewLogger(env Environment) xlog.Logger {
	config := zap.NewDevelopmentConfig()
	if env.IsDev() {
		config = zap.NewDevelopmentConfig()
	}

	config.EncoderConfig.TimeKey = "time"
	config.EncoderConfig.NameKey = "logger"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := config.Build(
		zap.AddCallerSkip(1),
	)
	if err != nil {
		err := fmt.Errorf("initialize logger failed: %w", err)
		log.Panic(err.Error())
	}
	return xlog.NewZapAdapter(logger.With(
		zap.String("app", buildmeta.GetAppBuildMeta().AppName),
		zap.String("version", buildmeta.GetAppBuildMeta().Version),
	))
}
