# xlog интеграция

## xlog пакет

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

## Использование xlog в store

```go
func (s *Store) Get(ctx context.Context, maintID uuid.UUID) (*entity.Maintenance, error) {
    ctx = xlog.WithOperation(ctx, "store.Maintenances.Get")

    stmt := table.Maintenances.
        SELECT(table.Maintenances.AllColumns).
        WHERE(table.Maintenances.ID.EQ(postgres.UUID(maintID)))

    maint := new(model.Maintenances)
    err := stmt.QueryContext(ctx, s.db.Executor(ctx), maint)
    if err != nil {
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
