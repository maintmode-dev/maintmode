---
name: echo-v5-framework
description: Echo v5 HTTP API patterns for MaintMode. Use when adding or changing Echo v5 handlers, route registration, middleware, request binding, response shaping, centralized error handling, or tests in the MaintMode app/api and server layers.
---

# Echo v5 Framework

Use Echo v5 only. MaintMode imports `github.com/labstack/echo/v5`; do not introduce Echo v4 APIs or `echo.Context` value handler signatures.

## MaintMode Entry Points

- Public API handlers live under `internal/app/api/public`.
- UI API handlers live under `internal/app/api/ui`.
- Infra handlers live under `internal/app/api/infra`.
- Server wiring lives under `internal/server`.
- API error mapping lives under `internal/app/api/apierrors`.

## Handler Pattern

```go
func (i *Implementation) GetResource(c *echo.Context) error {
    ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Resources.Get")
    defer span.End()

    op := "get resource"

    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ErrInvalidUUID)
        return c.JSON(statusCode, errResp)
    }

    resource, err := i.service.GetResource(ctx, id)
    if err != nil {
        statusCode, errResp := apierrors.ToAPIErrResponse(op, err)
        return c.JSON(statusCode, errResp)
    }

    return c.JSON(http.StatusOK, models.FromResource(resource))
}
```

Keep handlers thin: parse or bind input, call services, map errors, and serialize responses. Put business rules in services and SQL in stores.

## Binding And Validation

- Prefer existing `models.Bind*` helpers when the package already has them.
- Validate transport shape at the API boundary.
- Convert request models to entity commands before calling services.
- Return project API error responses via `apierrors.ToAPIErrResponse(op, err)` instead of raw Echo errors when behavior is user-visible.

## Middleware And Context

- Use `*echo.Context`; Echo v5 context is a concrete struct.
- Propagate `c.Request().Context()` into services and stores.
- Preserve auth and user data through existing middlewares in `internal/server/middlewares`.
- Add operation spans/log fields with `github.com/ruko1202/xlog` when matching nearby handlers.
- Echo v5 `c.Response()` returns `http.ResponseWriter`; use `c.Response().Header()` for headers and Echo v5 helpers such as `echo.UnwrapResponse` or `echo.ResolveResponseStatus` when response internals are needed.

## Testing

- For handler tests, use Echo v5 request/response primitives and project test helpers when present.
- For API behavior, prefer existing `test/api` generated client patterns.
- Cover status code, response model, and mapped error cases.
