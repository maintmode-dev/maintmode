---
name: error-handling-go
description: MaintMode Go error handling patterns for domain errors, storage error translation, service wrapping, Echo v4 API error mapping, and xlog context logging. Use when adding or reviewing errors across internal/apperr, services, storages, and internal/app/api/apierrors.
---

# MaintMode Error Handling

Use this skill when errors cross MaintMode layers. The goal is to preserve Go error chains internally and return stable API responses at the transport boundary.

## Repository Anchors

- Domain errors: `internal/apperr/errors.go`
- API mapper: `internal/app/api/apierrors/mapper.go`
- API validation helpers: `internal/app/api/apierrors/errors.go`
- API response type: `internal/app/api/apierrors/error_resp.go`

## Layer Rules

### Stores

- Convert `qrm.ErrNoRows` to a concrete `apperr` sentinel.
- Use Jet/qrm/sqlx patterns through `s.db.Executor(ctx)`.
- Add operation spans with `xlog` when matching nearby store code.

```go
item := new(model.Resources)
err := stmt.QueryContext(ctx, s.db.Executor(ctx), item)
if err != nil {
    if errors.Is(err, qrm.ErrNoRows) {
        return nil, apperr.ErrResourceNotFound
    }
    return nil, fmt.Errorf("query resource: %w", err)
}
```

### Services

- Validate commands before starting transactions.
- Return domain errors from `apperr`.
- Wrap unexpected dependency failures with `%w` when the added operation context helps debugging.
- Do not convert errors to HTTP responses in services.

```go
if err := validateCreate(cmd); err != nil {
    return nil, err
}

resource, err := s.store.GetByID(ctx, id)
if err != nil {
    return nil, fmt.Errorf("get resource: %w", err)
}
```

### Echo Handlers

- Parse and bind request data.
- Convert parse/validation failures to `apierrors.ErrInvalidUUID`, `apierrors.ErrParseBody`, or `apierrors.ValidationErr(err)`.
- Call services.
- Convert all service errors with `apierrors.ToAPIErrResponse(op, err)`.

```go
op := "get resource"

id, err := uuid.Parse(c.Param("id"))
if err != nil {
    statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ErrInvalidUUID)
    return c.JSON(statusCode, errResp)
}

resource, err := i.resourceSrv.GetResourceByID(ctx, id)
if err != nil {
    statusCode, errResp := apierrors.ToAPIErrResponse(op, err)
    return c.JSON(statusCode, errResp)
}
```

## Current Mapping Expectations

- Not found: `apperr.ErrMaintNotFound`, `apperr.ErrResourceNotFound` -> 404.
- Validation: `apperr.ErrValidation` and wrapped validation errors -> 400.
- Conflicts: maintenance/resource/conflict state errors -> 409 where mapped.
- Auth/token errors: use the auth branch in `apierrors.ToAPIErrResponse`.
- Unknown errors -> 500 with operation-level message.

## Logging

- Use `github.com/ruko1202/xlog` and `xfield`.
- Log where the layer has useful context; avoid duplicating the same low-level error at every layer.
- Never log tokens, cookies, OAuth codes, DSNs, raw auth headers, or secrets.

## Testing

- Use `errors.Is` for domain error assertions.
- Test handler status codes through `apierrors.ToAPIErrResponse` behavior.
- For store not-found paths, assert `qrm.ErrNoRows` is translated to the expected `apperr`.
