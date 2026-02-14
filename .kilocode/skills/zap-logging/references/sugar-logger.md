# Sugar Logger

## Создание Sugar logger

```go
import "go.uber.org/zap"

logger := zap.NewProduction()
sugar := logger.Sugar()
defer sugar.Sync()
```

## Форматированное логирование

### Printf-style форматирование

```go
sugar.Infof("User %s logged in from %s", userID, ipAddress)
sugar.Warnf("Request took %d ms", duration)
sugar.Errorf("Failed to process request: %v", err)
```

### Уровни логирования

```go
sugar.Debug("Debug message")
sugar.Info("Info message")
sugar.Warn("Warning message")
sugar.Error("Error message")
sugar.Fatal("Fatal message") // вызывает os.Exit(1)
sugar.Panic("Panic message") // вызывает panic()
```

## Structured logging с Sugar

### Infow, Errorw и др.

```go
sugar.Infow("User logged in",
    "user_id", userID,
    "ip_address", ipAddress,
    "timestamp", time.Now(),
)

sugar.Errorw("Database query failed",
    "error", err,
    "query", "SELECT * FROM users",
    "duration_ms", duration.Milliseconds(),
)
```

## Когда использовать Sugar vs обычный Logger

### Используйте Sugar когда:
- ✅ Нужен удобный Printf-style синтаксис
- ✅ Производительность не критична (разница ~50% в скорости)
- ✅ Быстрое прототипирование
- ✅ Простое логирование без сложных структур

### Используйте обычный Logger когда:
- ✅ Критична максимальная производительность
- ✅ Строгая типизация полей
- ✅ Zero-allocation logging
- ✅ Production код с высокой нагрузкой

## Преимущества Sugar

### Удобный синтаксис

```go
// Sugar - проще
sugar.Infof("User %s created", userID)

// Logger - многословнее
logger.Info("User created",
    zap.String("user_id", userID),
)
```

### Structured logging

```go
// Sugar - key-value пары
sugar.Infow("Request completed",
    "method", r.Method,
    "path", r.URL.Path,
    "status", statusCode,
)

// Logger - эквивалент
logger.Info("Request completed",
    zap.String("method", r.Method),
    zap.String("path", r.URL.Path),
    zap.Int("status", statusCode),
)
```

## Использование в handlers

```go
func (h *Handler) HandleRequest(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    sugar := xlog.FromContext(ctx).Sugar()

    sugar.Infof("Processing %s request to %s", r.Method, r.URL.Path)

    // ... обработка запроса

    sugar.Infow("Request completed",
        "status", statusCode,
        "duration_ms", duration.Milliseconds(),
    )
}
```

## Конвертация между Logger и Sugar

```go
// Logger -> Sugar
logger := zap.NewProduction()
sugar := logger.Sugar()

// Sugar -> Logger (не поддерживается напрямую)
// Нужно сохранить оригинальный logger
logger := zap.NewProduction()
sugar := logger.Sugar()
// Используйте logger для строго типизированного логирования
```

## Performance сравнение

| Операция | Logger (ns/op) | Sugar (ns/op) | Разница |
|----------|----------------|---------------|---------|
| Simple log | 800 | 1200 | +50% |
| With fields | 1500 | 2000 | +33% |

**Вывод:** Sugar на 30-50% медленнее, но удобнее в использовании.
