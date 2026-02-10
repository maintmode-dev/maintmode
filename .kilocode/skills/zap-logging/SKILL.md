---
name: zap-logging
description: Структурированное логирование с zap (sugar, logger, fields, rotation). Используй этот скилл, когда нужно настраивать структурированное логирование с zap, использовать sugar logger, добавлять fields и настраивать rotation.
license: MIT
metadata:
  category: development
  source:
    repository: project-specific
    path: zap-logging
---

# Структурированное логирование с Zap

## Описание
Этот скилл предоставляет руководство по использованию zap - высокопроизводительного структурированного логера для Go. Включает настройку logger и sugar, работу с fields, rotation и интеграцию с xlog.

## Когда использовать
Используй этот скилл, когда нужно:
- Настраивать структурированное логирование с zap
- Использовать sugar logger для удобного логирования
- Добавлять structured fields к логам
- Настраивать rotation логов
- Интегрировать zap с xlog
- Настраивать логирование для dev и production окружений

## Установка Zap

### Установка

```bash
go get go.uber.org/zap
go get go.uber.org/zap/zapcore
```

### Зависимости проекта

```go
import (
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)
```

## Базовая настройка logger

### Создание logger

Создайте файл `internal/config/logger.go`:

```go
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
```

### Environment тип

```go
type Environment string

const (
    EnvironmentDev     Environment = "dev"
    EnvironmentProd    Environment = "production"
    EnvironmentTest   Environment = "test"
)

func (e Environment) IsDev() bool {
    return e == EnvironmentDev
}

func (e Environment) IsProd() bool {
    return e == EnvironmentProd
}

func (e Environment) IsTest() bool {
    return e == EnvironmentTest
}
```

## Использование Logger

### Базовое логирование

```go
import "go.uber.org/zap"

logger := zap.NewExample()

// Debug
logger.Debug("Debug message")

// Info
logger.Info("Info message")

// Warn
logger.Warn("Warning message")

// Error
logger.Error("Error message", zap.Error(err))

// Fatal
logger.Fatal("Fatal message", zap.Error(err))
```

### Логирование с fields

```go
// Строковое поле
logger.Info("User created",
    zap.String("user_id", "123"),
    zap.String("username", "john_doe"),
)

// Числовое поле
logger.Info("Request processed",
    zap.Int("status_code", 200),
    zap.Duration("duration", time.Millisecond*150),
)

// Булево поле
logger.Info("Feature enabled",
    zap.Bool("enabled", true),
)

// Любой тип
logger.Info("Complex data",
    zap.Any("data", map[string]interface{}{
        "key1": "value1",
        "key2": 42,
    }),
)
```

### Логирование ошибок

```go
err := doSomething()

// Логирование ошибки как field
logger.Error("Operation failed",
    zap.Error(err),
    zap.String("operation", "create_user"),
)

// Логирование стека
logger.Error("Operation failed with stack",
    zap.Error(err),
    zap.Stack("stack"),
)

// Логирование ошибки с дополнительными полями
logger.Error("Database query failed",
    zap.Error(err),
    zap.String("query", "SELECT * FROM users"),
    zap.Duration("duration", time.Millisecond*50),
)
```

## Sugar Logger

### Создание Sugar logger

```go
sugar := logger.Sugar()

// Использование sugar
sugar.Debug("Debug message")
sugar.Infof("User %s logged in", userID)
sugar.Warnf("Connection timeout: %v", timeout)
sugar.Errorf("Failed to connect: %v", err)
```

### Преимущества Sugar

Sugar logger предоставляет более удобный интерфейс:

```go
// Вместо
logger.Info("User created",
    zap.String("user_id", "123"),
    zap.String("username", "john_doe"),
)

// Можно написать
sugar.Infow("User created",
    "user_id", "123",
    "username", "john_doe",
)

// Или даже проще
sugar.Infof("User %s created", userID)
```

### Использование Sugar в handlers

```go
func (h *Handler) HandleRequest(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    logger := xlog.FromContext(ctx).Sugar()

    logger.Infof("Processing request: %s %s", r.Method, r.URL.Path)

    // ... обработка запроса

    logger.Infof("Request completed with status: %d", statusCode)
}
```

## xlog интеграция

### xlog пакет

Создайте файл `internal/utils/xlog/xlog.go`:

```go
package xlog

import (
    "context"

    "go.uber.org/zap"
)

// Context key for logger
type contextKey struct{}

// FromContext returns logger from context or default logger
func FromContext(ctx context.Context) *zap.Logger {
    if l, ok := ctx.Value(contextKey{}).(*zap.Logger); ok {
        return l
    }
    return zap.NewNop()
}

// WithContext returns context with logger
func WithContext(ctx context.Context, logger *zap.Logger) context.Context {
    return context.WithValue(ctx, contextKey{}, logger)
}

// WithOperation adds operation field to logger
func WithOperation(ctx context.Context, operation string) context.Context {
    logger := FromContext(ctx).With(
        zap.String("operation", operation),
    )
    return WithContext(ctx, logger)
}

// WithRequestID adds request_id field to logger
func WithRequestID(ctx context.Context, requestID string) context.Context {
    logger := FromContext(ctx).With(
        zap.String("request_id", requestID),
    )
    return WithContext(ctx, logger)
}

// WithUserID adds user_id field to logger
func WithUserID(ctx context.Context, userID string) context.Context {
    logger := FromContext(ctx).With(
        zap.String("user_id", userID),
    )
    return WithContext(ctx, logger)
}
```

### Использование xlog в store

```go
func (s *Store) Get(ctx context.Context, maintID uuid.UUID) (*entity.Maintenance, error) {
    ctx = xlog.WithOperation(ctx, "store.Maintenances.Get")

    stmt := table.Maintenances.
        SELECT(table.Maintenances.AllColumns).
        WHERE(table.Maintenances.ID.EQ(postgres.UUID(maintID)))

    maint := new(model.Maintenances)
    err := stmt.QueryContext(ctx, s.db.Executor(ctx), maint)
    if err != nil {
        if errors.Is(err, qrm.ErrNoRows) {
            xlog.FromContext(ctx).Error("Maintenance not found",
                zap.String("maint_id", maintID.String()),
                zap.Error(err),
            )
            return nil, apperr.ErrMaintNotFound
        }
        xlog.FromContext(ctx).Error("Failed to get maintenance",
            zap.String("maint_id", maintID.String()),
            zap.Error(err),
        )
        return nil, err
    }

    xlog.FromContext(ctx).Info("Maintenance retrieved successfully",
        zap.String("maint_id", maintID.String()),
    )

    return fromDBMaintenance(maint), nil
}
```

### Использование xlog в service

```go
func (s *Service) CreateDraft(ctx context.Context, req *CreateDraftRequest) (*entity.Maintenance, error) {
    ctx = xlog.WithOperation(ctx, "service.Maintenances.CreateDraft")

    maint, err := s.store.Create(ctx, req)
    if err != nil {
        xlog.FromContext(ctx).Error("Failed to create draft maintenance",
            zap.Error(err),
        )
        return nil, err
    }

    xlog.FromContext(ctx).Info("Draft maintenance created successfully",
        zap.String("maint_id", maint.ID.String()),
        zap.String("title", maint.Title),
    )

    return maint, nil
}
```

## Конфигурация для разных окружений

### Development конфигурация

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

### Production конфигурация

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

## Rotation логов

### Использование lumberjack для rotation

Установка:

```bash
go get gopkg.in/natefinch/lumberjack.v2
```

### Настройка rotation

```go
import (
    "gopkg.in/natefinch/lumberjack.v2"
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

func NewLoggerWithRotation(logPath string) *zap.Logger {
    writer := &lumberjack.Logger{
        Filename:   logPath,
        MaxSize:    100, // megabytes
        MaxBackups: 3,
        MaxAge:     28, // days
        Compress:   true,
    }

    core := zapcore.NewCore(
        zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
        zapcore.AddSync(writer),
        zapcore.InfoLevel,
    )

    return zap.New(core, zap.AddCaller())
}
```

### Настройка rotation для dev и prod

```go
func NewLogger(env Environment) *zap.Logger {
    var config zap.Config

    if env.IsDev() {
        config = zap.NewDevelopmentConfig()
        config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
    } else {
        config = zap.NewProductionConfig()

        // Add rotation for production
        writer := &lumberjack.Logger{
            Filename:   "logs/app.log",
            MaxSize:    100,
            MaxBackups: 3,
            MaxAge:     28,
            Compress:   true,
        }

        config.OutputPaths = []string{"stdout", "logs/app.log"}
        config.ErrorOutputPaths = []string{"stderr", "logs/error.log"}
    }

    config.EncoderConfig.TimeKey = "time"
    config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

    logger, _ := config.Build(
        zap.AddCallerSkip(1),
    )
    return logger
}
```

## Structured Fields

### Стандартные поля

```go
// Request ID
zap.String("request_id", requestID)

// Operation
zap.String("operation", "user.create")

// User ID
zap.String("user_id", userID)

// Duration
zap.Duration("duration", time.Since(start))

// Status code
zap.Int("status_code", statusCode)

// Error
zap.Error(err)

// Stack trace
zap.Stack("stack")
```

### Кастомные поля

```go
// Enum типы
zap.String("status", string(entity.StatusActive))

// UUID
zap.String("id", uuid.New().String())

// JSON данные
zap.String("payload", string(jsonData))

// Массивы
zap.Strings("tags", []string{"tag1", "tag2"})

// Вложенные данные
zap.Any("metadata", map[string]interface{}{
    "key1": "value1",
    "key2": 42,
})
```

## Middleware для HTTP

### Логирование HTTP запросов

```go
func LoggingMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()

            // Добавляем logger в контекст
            ctx := xlog.WithRequestID(r.Context(), generateRequestID())
            r = r.WithContext(ctx)

            // Создаем response writer для захвата статуса
            ww := &responseWriter{ResponseWriter: w}

            next.ServeHTTP(ww, r)

            // Логируем запрос
            logger.Info("HTTP request",
                zap.String("method", r.Method),
                zap.String("path", r.URL.Path),
                zap.Int("status", ww.statusCode),
                zap.Duration("duration", time.Since(start)),
                zap.String("request_id", xlog.FromContext(ctx).String()),
            )
        })
    }
}

type responseWriter struct {
    http.ResponseWriter
    statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}
```

## Полный пример конфигурации

```go
package config

import (
    "fmt"
    "log"

    "gopkg.in/natefinch/lumberjack.v2"
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

func NewLogger(env Environment) *zap.Logger {
    var config zap.Config

    if env.IsDev() {
        config = zap.NewDevelopmentConfig()
        config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
        config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
    } else {
        config = zap.NewProductionConfig()

        // Настройка rotation для production
        writer := &lumberjack.Logger{
            Filename:   "logs/app.log",
            MaxSize:    100, // megabytes
            MaxBackups: 3,
            MaxAge:     28, // days
            Compress:   true,
        }

        errorWriter := &lumberjack.Logger{
            Filename:   "logs/error.log",
            MaxSize:    100,
            MaxBackups: 3,
            MaxAge:     28,
            Compress:   true,
        }

        config.OutputPaths = []string{"stdout", "logs/app.log"}
        config.ErrorOutputPaths = []string{"stderr", "logs/error.log"}
    }

    config.EncoderConfig.TimeKey = "time"
    config.EncoderConfig.NameKey = "logger"
    config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

    logger, err := config.Build(
        zap.AddCallerSkip(1),
        zap.AddStacktrace(zapcore.ErrorLevel),
    )
    if err != nil {
        err := fmt.Errorf("initialize logger failed: %w", err)
        log.Panic(err.Error())
    }
    return logger
}
```

## Лучшие практики

1. **Используйте structured logging** - добавляйте поля вместо форматирования строк
2. **Используйте xlog для контекста** - передавайте logger через context
3. **Добавляйте operation field** - для отслеживания потока выполнения
4. **Логируйте ошибки с zap.Error()** - для корректного отображения
5. **Используйте rotation в production** - для управления размером логов
6. **Разделяйте dev и prod конфигурации** - для удобства разработки
7. **Добавляйте request_id** - для трассировки запросов
8. **Используйте sugar для простых случаев** - для удобного форматирования

## Полезные команды

### Просмотр логов

```bash
# Все логи
tail -f logs/app.log

# Только ошибки
tail -f logs/error.log

# Фильтрация по operation
grep "operation=user.create" logs/app.log

# Фильтрация по request_id
grep "request_id=abc123" logs/app.log
```

### JSON парсинг

```bash
# Красивый вывод JSON логов
cat logs/app.log | jq

# Фильтрация по полю
cat logs/app.log | jq '. | select(.level=="error")'
```

## Ресурсы

- [Zap Documentation](https://github.com/uber-go/zap)
- [Zap Best Practices](https://github.com/uber-go/zap/blob/master/FAQ.md)
- [Lumberjack Documentation](https://github.com/natefinch/lumberjack)
