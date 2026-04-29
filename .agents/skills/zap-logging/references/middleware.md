# Echo v4 Request Logging

Use this reference when changing request logging around Echo v4 middleware. Keep request logs low-cardinality and avoid sensitive values.

## Middleware Shape

```go
import (
    "time"

    "github.com/labstack/echo/v4"
    "github.com/ruko1202/xlog"
    "github.com/ruko1202/xlog/xfield"
)

func LoggingMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        startedAt := time.Now()
        ctx := c.Request().Context()

        err := next(c)

        fields := []xfield.Field{
            xfield.String("method", c.Request().Method),
            xfield.String("path", c.Path()),
            xfield.Int("status", c.Response().Status),
            xfield.Duration("duration", time.Since(startedAt)),
        }

        if err != nil || c.Response().Status >= 500 {
            xlog.Error(ctx, "http request failed", append(fields, xfield.Error(err))...)
            return err
        }
        if c.Response().Status >= 400 {
            xlog.Warn(ctx, "http request error", fields...)
            return err
        }

        xlog.Info(ctx, "http request completed", fields...)
        return err
    }
}
```

## Rules

- Use `c.Path()` rather than raw `URL.Path` to avoid high-cardinality IDs in labels/log fields.
- Do not log headers or cookies by default.
- Include request IDs only if existing middleware has already generated and sanitized them.
- Preserve Echo error flow by returning `err` from `next(c)`.
