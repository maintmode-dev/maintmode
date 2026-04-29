---
name: zap-logging
description: MaintMode structured logging with zap through github.com/ruko1202/xlog. Use when adding operation logs, request-scoped fields, error logs, tracing spans, logger configuration, or Echo v4 HTTP logging in services, stores, handlers, and app startup.
---

# Zap Logging

MaintMode uses `github.com/ruko1202/xlog` as the logging and tracing facade over zap. Prefer `xlog` and `xfield` in application code instead of using raw `zap.Logger` directly.

## Service Or Store Pattern

```go
import (
    "github.com/ruko1202/xlog"
    "github.com/ruko1202/xlog/xfield"
)

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*entity.Resource, error) {
    ctx, span := xlog.WithOperationSpan(ctx, "service.Resources.Get")
    defer span.End()

    resource, err := s.store.Get(ctx, id)
    if err != nil {
        xlog.Error(ctx, "failed to get resource",
            xfield.String("resource_id", id.String()),
            xfield.Error(err),
        )
        return nil, fmt.Errorf("get resource: %w", err)
    }

    return resource, nil
}
```

## Handler Pattern

```go
func (i *Implementation) Get(c echo.Context) error {
    ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Resources.Get")
    defer span.End()
    op := "get resource"

    result, err := i.service.Get(ctx, id)
    if err != nil {
        xlog.Error(ctx, "failed to get resource", xfield.Error(err))
        statusCode, errResp := apierrors.ToAPIErrResponse(op, err)
        return c.JSON(statusCode, errResp)
    }

    return c.JSON(http.StatusOK, result)
}
```

## Rules

- Use operation names that match the layer: `api.*`, `service.*`, `store.*`.
- Wrap returned errors with `%w`; logs do not replace error context.
- Log unexpected errors where the layer has useful business or request context.
- Use stable, low-cardinality fields such as IDs, statuses, actions, and operation names.
- Never log secrets, DSNs, access tokens, refresh tokens, OAuth codes, raw cookies, or full auth headers.

## References

- Read [references/logger-configuration.md](references/logger-configuration.md) when changing app startup logger configuration.
- Read [references/structured-fields.md](references/structured-fields.md) when choosing field names and field types.
- Read [references/xlog-integration.md](references/xlog-integration.md) for MaintMode context and span examples.
- Read [references/middleware.md](references/middleware.md) when changing Echo v4 request logging.
