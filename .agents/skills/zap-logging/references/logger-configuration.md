# Конфигурация Logger

## Базовая настройка logger

```go
package config

import (
    "fmt"
    "log"

    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

func NewLogger(env Environment) *zap.Logger {
    var config zap.Config
    
    if env.IsDev() {
        config = zap.NewDevelopmentConfig()
        config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
    } else {
        config = zap.NewProductionConfig()
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
```

## Development конфигурация

```go
func NewDevLogger() *zap.Logger {
    config := zap.NewDevelopmentConfig()
    config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
    config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
    config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

    logger, _ := config.Build(
        zap.AddCaller(),
        zap.AddCallerSkip(1),
    )
    return logger
}
```

## Production конфигурация

```go
func NewProdLogger() *zap.Logger {
    config := zap.NewProductionConfig()
    config.EncoderConfig.TimeKey = "time"
    config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
    config.EncoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder

    logger, _ := config.Build(
        zap.AddCaller(),
        zap.AddCallerSkip(1),
        zap.AddStacktrace(zapcore.ErrorLevel),
    )
    return logger
}
```
