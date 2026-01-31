package config

import (
	"fmt"
	"log"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger(env Environment) *zap.Logger {
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
	return logger
}
