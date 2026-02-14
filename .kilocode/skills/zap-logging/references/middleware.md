# HTTP Middleware для логирования

## Базовый logging middleware

```go
func LoggingMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()

            // Response writer wrapper для захвата status code
            ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

            // Обработка запроса
            next.ServeHTTP(ww, r)

            // Логирование после обработки
            logger.Info("HTTP request",
                zap.String("method", r.Method),
                zap.String("path", r.URL.Path),
                zap.Int("status", ww.statusCode),
                zap.Duration("duration", time.Since(start)),
                zap.String("remote_addr", r.RemoteAddr),
            )
        })
    }
}

// Response writer wrapper
type responseWriter struct {
    http.ResponseWriter
    statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}
```

## Middleware с request_id

```go
func RequestIDMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Генерация request ID
            requestID := generateRequestID()

            // Добавление logger с request_id в контекст
            ctx := xlog.WithRequestID(r.Context(), requestID)
            r = r.WithContext(ctx)

            // Добавление request_id в response header
            w.Header().Set("X-Request-ID", requestID)

            next.ServeHTTP(w, r)
        })
    }
}

func generateRequestID() string {
    return uuid.New().String()
}
```

## Integration с Echo

```go
import (
    "github.com/labstack/echo/v5"
    "go.uber.org/zap"
)

func ZapMiddleware(logger *zap.Logger) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c *echo.Context) error {
            start := time.Now()

            // Добавление logger в контекст
            ctx := xlog.WithContext(c.Request().Context(), logger)
            c.SetRequest(c.Request().WithContext(ctx))

            // Обработка запроса
            err := next(c)

            // Логирование
            logger.Info("Request processed",
                zap.String("method", c.Request().Method),
                zap.String("path", c.Request().URL.Path),
                zap.Int("status", c.Response().Status),
                zap.Duration("latency", time.Since(start)),
            )

            return err
        }
    }
}

// Использование
e := echo.New()
e.Use(ZapMiddleware(logger))
```

## Integration с Gin

```go
import (
    "github.com/gin-gonic/gin"
    "go.uber.org/zap"
)

func GinZapMiddleware(logger *zap.Logger) gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()

        // Обработка запроса
        c.Next()

        // Логирование
        latency := time.Since(start)
        statusCode := c.Writer.Status()

        logger.Info("Request processed",
            zap.String("method", c.Request.Method),
            zap.String("path", c.Request.URL.Path),
            zap.Int("status", statusCode),
            zap.Duration("latency", latency),
            zap.String("client_ip", c.ClientIP()),
        )
    }
}

// Использование
r := gin.New()
r.Use(GinZapMiddleware(logger))
```

## Integration с Chi

```go
import (
    "github.com/go-chi/chi/v5"
    "go.uber.org/zap"
)

func ChiZapMiddleware(logger *zap.Logger) func(next http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()

            ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

            // Добавление logger в контекст
            ctx := xlog.WithContext(r.Context(), logger)
            r = r.WithContext(ctx)

            next.ServeHTTP(ww, r)

            logger.Info("Request completed",
                zap.String("method", r.Method),
                zap.String("path", r.URL.Path),
                zap.Int("status", ww.Status()),
                zap.Int("bytes", ww.BytesWritten()),
                zap.Duration("duration", time.Since(start)),
            )
        })
    }
}

// Использование
r := chi.NewRouter()
r.Use(ChiZapMiddleware(logger))
```

## Расширенный middleware с error handling

```go
func AdvancedLoggingMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            requestID := generateRequestID()

            // Logger с request_id
            reqLogger := logger.With(
                zap.String("request_id", requestID),
                zap.String("method", r.Method),
                zap.String("path", r.URL.Path),
            )

            // Добавление в контекст
            ctx := xlog.WithContext(r.Context(), reqLogger)
            r = r.WithContext(ctx)

            // Response wrapper
            ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
            w.Header().Set("X-Request-ID", requestID)

            // Логирование начала
            reqLogger.Debug("Request started")

            // Обработка с recovery
            defer func() {
                if rec := recover(); rec != nil {
                    reqLogger.Error("Panic recovered",
                        zap.Any("panic", rec),
                        zap.Stack("stack"),
                    )
                    http.Error(w, "Internal Server Error", http.StatusInternalServerError)
                }
            }()

            next.ServeHTTP(ww, r)

            // Логирование завершения
            duration := time.Since(start)
            if ww.statusCode >= 500 {
                reqLogger.Error("Request failed",
                    zap.Int("status", ww.statusCode),
                    zap.Duration("duration", duration),
                )
            } else if ww.statusCode >= 400 {
                reqLogger.Warn("Request error",
                    zap.Int("status", ww.statusCode),
                    zap.Duration("duration", duration),
                )
            } else {
                reqLogger.Info("Request completed",
                    zap.Int("status", ww.statusCode),
                    zap.Duration("duration", duration),
                )
            }
        })
    }
}
```

## Использование в handlers

После настройки middleware, logger доступен через контекст:

```go
func YourHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    logger := xlog.FromContext(ctx)

    logger.Info("Processing request")

    // Ваша бизнес-логика

    logger.Info("Request processed successfully")
}
```

## Best Practices

1. **Добавляйте request_id** - для трассировки запросов
2. **Логируйте разные уровни** - Info для success, Warn для 4xx, Error для 5xx
3. **Измеряйте latency** - используйте Duration для отслеживания производительности
4. **Recovery в middleware** - перехватывайте panic
5. **Logger в контексте** - передавайте через context для доступа в handlers
