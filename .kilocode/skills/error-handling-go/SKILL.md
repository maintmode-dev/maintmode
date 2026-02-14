---
name: error-handling-go
description: Error handling patterns and best practices for Go applications. Use when implementing error handling in Go projects, creating custom error types, converting domain errors to HTTP responses, error wrapping across layers, error logging with context, handling errors in Echo middleware, or establishing error handling architecture. Covers domain vs transport error separation, error hierarchies, sentinel errors, and layer-specific patterns (app/services/storages).
license: MIT
metadata:
  category: development
  source:
    repository: project-specific
    path: error-handling-go
---

# Error Handling in Go - Best Practices

## Core Principles

### 1. Domain vs Transport Error Separation

**Domain Errors**: Business logic errors that exist independent of transport layer
- `ErrUserNotFound`
- `ErrInvalidInput`
- `ErrConflict`

**Transport Errors**: HTTP-specific representations
- `ErrorResponse{Code: "not_found", Message: "User not found"}`
- Status codes (404, 409, 422)

**Rule**: Domain layer returns domain errors. HTTP layer converts them to transport errors.

### 2. Error Wrapping for Context

Wrap errors as they propagate through layers to preserve context:

```go
// Storage layer
user, err := r.db.Get(ctx, id)
if err != nil {
    return nil, fmt.Errorf("get user from db: %w", err)
}

// Service layer
user, err := s.repo.GetUser(ctx, id)
if err != nil {
    return nil, fmt.Errorf("fetch user: %w", err)
}
```

Use `%w` verb to preserve error chain for `errors.Is()` and `errors.As()`.

### 3. Sentinel Errors Pattern

Define package-level error variables for common errors:

```go
package apperr

import "errors"

var (
    ErrNotFound      = errors.New("resource not found")
    ErrConflict      = errors.New("resource conflict")
    ErrUnauthorized  = errors.New("unauthorized")
    ErrForbidden     = errors.New("forbidden")
    ErrValidation    = errors.New("validation failed")
)
```

Check with `errors.Is()`:

```go
if errors.Is(err, apperr.ErrNotFound) {
    return http.StatusNotFound
}
```

## Project Structure

For MaintMode project with existing error handling:

```
internal/
├── apperr/               # Domain errors
│   └── errors.go         # Sentinel errors, custom error types
├── app/api/
│   └── apierrors/        # Transport errors
│       └── errors.go     # ErrorResponse, ErrorCode types
└── services/             # Business logic
    └── user/
        └── service.go    # Returns domain errors
```

## Layer-Specific Patterns

### Storage Layer

**Pattern**: Wrap database errors, convert to domain errors when appropriate

```go
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*User, error) {
    var user User
    err := r.db.QueryRow(ctx, "SELECT * FROM users WHERE id = $1", id).Scan(&user)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, apperr.ErrNotFound
        }
        return nil, fmt.Errorf("query user: %w", err)
    }
    return &user, nil
}
```

### Service Layer

**Pattern**: Business logic validation, wrap errors with operation context

```go
func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (*User, error) {
    ctx = xlog.WithOperation(ctx, "service.User.GetUser")

    user, err := s.repo.Get(ctx, id)
    if err != nil {
        if errors.Is(err, apperr.ErrNotFound) {
            xlog.Warn(ctx, "user not found", zap.String("id", id.String()))
            return nil, err
        }
        xlog.Error(ctx, "get user failed", zap.Error(err))
        return nil, fmt.Errorf("get user: %w", err)
    }

    return user, nil
}
```

### HTTP Handler Layer (Echo)

**Pattern**: Convert domain errors to HTTP responses, handle validation

```go
func (h *Handler) GetUser(c echo.Context) error {
    ctx := xlog.WithOperation(c.Request().Context(), "api.GetUser")

    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        xlog.Error(ctx, "parse uuid failed", zap.Error(err))
        return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
            apierrors.ErrInvalidRequest,
            "id must be a valid UUID",
        ))
    }

    user, err := h.service.GetUser(ctx, id)
    if err != nil {
        xlog.Error(ctx, "get user failed", zap.Error(err))
        if errors.Is(err, apperr.ErrNotFound) {
            return c.JSON(http.StatusNotFound, apierrors.NewErrorResponse(
                apierrors.ErrNotFound,
                err.Error(),
            ))
        }

        return c.JSON(http.StatusInternalServerError, apierrors.NewErrorResponse(
            apierrors.ErrInternalError,
            "get user failed",
        ))
    }

    return c.JSON(http.StatusOK, user)
}
```

## Error Types

### 1. Simple Sentinel Errors

Best for: Common, general-purpose errors

```go
var ErrNotFound = errors.New("not found")
```

### 2. Parameterized Errors

Best for: Errors needing dynamic context

```go
var ErrForbiddenStatusTransition = errors.New("forbidden status transition")

func ForbiddenStatusTransition(currentStatus string) error {
    return fmt.Errorf("%w: %s", ErrForbiddenStatusTransition, currentStatus)
}
```

Usage:
```go
return apperr.ForbiddenStatusTransition("draft")
// Error: "forbidden status transition: draft"

// Check with errors.Is()
if errors.Is(err, apperr.ErrForbiddenStatusTransition) {
    // Handle forbidden transition
}
```

### 3. Custom Error Types

Best for: Complex errors with structured data

```go
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed for field %s: %s", e.Field, e.Message)
}

// Check with errors.As()
var validationErr *ValidationError
if errors.As(err, &validationErr) {
    // Access validationErr.Field, validationErr.Message
}
```

## HTTP Error Mapping

### Domain Error to HTTP Status

Common mappings:

| Domain Error | HTTP Status | Error Code |
|--------------|-------------|------------|
| `ErrNotFound` | 404 | `not_found` |
| `ErrConflict` | 409 | `conflict` |
| `ErrValidation` | 422 | `validation_error` |
| `ErrUnauthorized` | 401 | `unauthorized` |
| `ErrForbidden` | 403 | `forbidden` |
| Other | 500 | `internal_error` |

### Error Response Structure

```go
type ErrorCode string

var (
    ErrNotFound       ErrorCode = "not_found"
    ErrInternalError  ErrorCode = "internal_error"
    ErrInvalidRequest ErrorCode = "invalid_request"
)

type ErrorResponse struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

func NewErrorResponse(code ErrorCode, message string) *ErrorResponse {
    return &ErrorResponse{
        Code:    string(code),
        Message: message,
    }
}
```

## Error Logging Strategy

### When to Log

- **Always log** at HTTP handler layer (entry point)
- **Log warnings** for expected errors (not found, validation)
- **Log errors** for unexpected errors (database, external services)
- **Don't log** at every layer (prevents duplication)

### Context Enrichment

```go
ctx = xlog.WithOperation(ctx, "service.User.Create")
ctx = xlog.WithField(ctx, "user_id", userID)

xlog.Error(ctx, "operation failed",
    zap.Error(err),
    zap.String("email", email),
    zap.Duration("elapsed", time.Since(start)),
)
```

### Logging Levels

- **Error**: Unexpected errors requiring investigation
- **Warn**: Expected errors (not found, validation failures)
- **Info**: Successful operations
- **Debug**: Detailed debugging information

## Complete Examples

See [references/examples.md](references/examples.md) for:
- Complete service implementation with error handling
- Echo handler with comprehensive error conversion
- Custom error types with structured data
- Error handling in middleware
- Transaction error handling patterns
- Testing error scenarios

## Error Handling Patterns

See [references/domain-errors.md](references/domain-errors.md) for:
- Defining domain error hierarchies
- Creating error packages per domain
- Error wrapping strategies
- Sentinel error patterns

See [references/http-mapping.md](references/http-mapping.md) for:
- Comprehensive HTTP status code mappings
- Echo error middleware patterns
- Error response formatting
- Client-friendly error messages

## Best Practices

1. **Always use error wrapping** with `%w` to preserve error chains
2. **Define sentinel errors** at package level for common errors
3. **Separate domain and transport errors** clearly
4. **Log once** at the entry point (HTTP handler)
5. **Include operation context** in logs using `xlog.WithOperation`
6. **Return user-friendly messages** in HTTP responses
7. **Never expose internal errors** to clients
8. **Use `errors.Is()`** for sentinel error checks
9. **Use `errors.As()`** for custom error type checks
10. **Test error paths** thoroughly

## Common Pitfalls

1. **Don't return wrapped errors to clients**: Extract domain error first
2. **Don't log at every layer**: Log once at handler
3. **Don't lose error context**: Always wrap with `fmt.Errorf("%w")`
4. **Don't mix error types**: Keep domain errors in domain layer
5. **Don't ignore errors**: Handle or propagate, never ignore
6. **Don't use string comparison**: Use `errors.Is()` and `errors.As()`

## Migration from String Comparison

If you have legacy code using string comparison:

```go
// Bad
if err.Error() == "not found" {
    // ...
}

// Good
if errors.Is(err, apperr.ErrNotFound) {
    // ...
}
```

Migration steps:
1. Define sentinel errors in `apperr` package
2. Update storage layer to return sentinel errors
3. Update handlers to use `errors.Is()` checks
4. Update tests to use sentinel errors

## Testing Error Scenarios

```go
func TestService_GetUser_NotFound(t *testing.T) {
    repo := &mockRepo{
        err: apperr.ErrNotFound,
    }
    service := NewService(repo)

    _, err := service.GetUser(context.Background(), uuid.New())

    assert.True(t, errors.Is(err, apperr.ErrNotFound))
}

func TestHandler_GetUser_ReturnsNotFound(t *testing.T) {
    service := &mockService{
        err: apperr.ErrNotFound,
    }
    handler := NewHandler(service)

    req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
    rec := httptest.NewRecorder()
    c := echo.New().NewContext(req, rec)

    err := handler.GetUser(c)

    assert.NoError(t, err)
    assert.Equal(t, http.StatusNotFound, rec.Code)
}
```

## Integration with MaintMode Project

Your project already has good error handling foundation:

**Existing strengths:**
- Sentinel errors defined in `internal/apperr/errors.go`
- Error response types in `internal/app/api/apierrors/errors.go`
- Parameterized error functions (`ForbiddenStatusTransition`)
- Domain error checking in handlers with `errors.Is()`

**Recommendations:**
1. Standardize error wrapping across all layers
2. Add operation context to all service methods
3. Create helper function for domain-to-HTTP mapping
4. Document error codes for API consumers
5. Add error middleware for consistent logging
6. Consider error aggregation for batch operations

## Resources

- **Go Blog: Error Handling**: https://go.dev/blog/error-handling-and-go
- **Go Blog: Working with Errors**: https://go.dev/blog/go1.13-errors
- **pkg/errors** (alternative): https://github.com/pkg/errors
- **Echo Error Handling**: https://echo.labstack.com/docs/error-handling
