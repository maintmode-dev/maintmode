package config

import (
	"fmt"
	"log"

	"github.com/ruko1202/xlog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/ruko1202/maintmode/internal/config/buildmeta"
)

func NewLogger(env Environment, logCfg LoggerConfig, meta *buildmeta.AppBuildMeta) xlog.Logger {
	config := zap.NewProductionConfig()
	if env.IsLocal() {
		config = zap.NewDevelopmentConfig()
	}

	config.EncoderConfig.TimeKey = "time"
	config.EncoderConfig.NameKey = "logger"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.Level.SetLevel(zapcore.WarnLevel)

	logLevel, err := zapcore.ParseLevel(logCfg.Level)
	if err == nil {
		config.Level.SetLevel(logLevel)
	}

	logger, err := config.Build(
		zap.AddCallerSkip(2),
		zap.AddCaller(),
	)
	if err != nil {
		err := fmt.Errorf("initialize logger failed: %w", err)
		log.Panic(err.Error())
	}
	return xlog.NewZapAdapter(logger.With(
		zap.String("app", meta.AppName),
		zap.String("version", meta.Version),
	))
}
