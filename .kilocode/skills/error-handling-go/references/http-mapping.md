# HTTP Error Mapping

## Standard HTTP Status Codes

### 2xx Success
- **200 OK**: Request succeeded
- **201 Created**: Resource created successfully
- **202 Accepted**: Request accepted for processing
- **204 No Content**: Request succeeded, no content to return

### 4xx Client Errors
- **400 Bad Request**: Malformed request (invalid JSON, missing required fields)
- **401 Unauthorized**: Authentication required or failed
- **403 Forbidden**: Authenticated but not authorized
- **404 Not Found**: Resource not found
- **405 Method Not Allowed**: HTTP method not supported
- **409 Conflict**: Request conflicts with current state
- **422 Unprocessable Entity**: Validation failed
- **429 Too Many Requests**: Rate limit exceeded

### 5xx Server Errors
- **500 Internal Server Error**: Unexpected server error
- **502 Bad Gateway**: Invalid response from upstream
- **503 Service Unavailable**: Service temporarily unavailable
- **504 Gateway Timeout**: Upstream timeout

## Domain Error to HTTP Status Mapping

### Mapping Function Pattern

```go
package apierrors

import (
    "errors"
    "net/http"

    "github.com/ruko1202/maintmode/internal/apperr"
)

func DomainErrorToHTTPStatus(err error) int {
    switch {
    case errors.Is(err, apperr.ErrNotFound):
        return http.StatusNotFound
    case errors.Is(err, apperr.ErrMaintNotFound):
        return http.StatusNotFound
    case errors.Is(err, apperr.ErrResourceNotFound):
        return http.StatusNotFound

    case errors.Is(err, apperr.ErrValidation):
        return http.StatusUnprocessableEntity
    case errors.Is(err, apperr.ErrInvalidPeriodEmptyStartOrEnd):
        return http.StatusUnprocessableEntity
    case errors.Is(err, apperr.ErrInvalidPeriodStartOrEnd):
        return http.StatusUnprocessableEntity
    case errors.Is(err, apperr.ErrInvalidPeriodInterval):
        return http.StatusUnprocessableEntity

    case errors.Is(err, apperr.ErrConflict):
        return http.StatusConflict
    case errors.Is(err, apperr.ErrConflictsChangedSincePreview):
        return http.StatusConflict
    case errors.Is(err, apperr.ErrMaintChangedSincePreview):
        return http.StatusConflict

    case errors.Is(err, apperr.ErrForbidden):
        return http.StatusForbidden
    case errors.Is(err, apperr.ErrForbiddenStatusTransition):
        return http.StatusForbidden

    case errors.Is(err, apperr.ErrUnauthorized):
        return http.StatusUnauthorized

    default:
        return http.StatusInternalServerError
    }
}

func DomainErrorToErrorCode(err error) ErrorCode {
    switch {
    case errors.Is(err, apperr.ErrNotFound):
        return ErrNotFound
    case errors.Is(err, apperr.ErrValidation):
        return ErrInvalidRequest
    case errors.Is(err, apperr.ErrConflict):
        return ErrConflict
    case errors.Is(err, apperr.ErrForbidden):
        return ErrForbidden
    case errors.Is(err, apperr.ErrUnauthorized):
        return ErrUnauthorized
    default:
        return ErrInternalError
    }
}
```

### Handler Helper Pattern

```go
package apierrors

import (
    "github.com/labstack/echo/v4"
    "github.com/ruko1202/xlog"
    "go.uber.org/zap"
)

// HandleError converts domain errors to HTTP responses
func HandleError(c echo.Context, err error, operation string) error {
    ctx := c.Request().Context()

    status := DomainErrorToHTTPStatus(err)
    code := DomainErrorToErrorCode(err)

    // Log based on status code
    if status >= 500 {
        xlog.Error(ctx, operation+" failed", zap.Error(err))
    } else if status >= 400 {
        xlog.Warn(ctx, operation+" client error", zap.Error(err), zap.Int("status", status))
    }

    // Return user-friendly message for internal errors
    message := err.Error()
    if status == 500 {
        message = operation + " failed"
    }

    return c.JSON(status, NewErrorResponse(code, message))
}
```

**Usage in handlers:**
```go
func (h *Handler) GetMaintenance(c echo.Context) error {
    ctx := c.Request().Context()

    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
            apierrors.ErrInvalidRequest,
            "id must be a valid UUID",
        ))
    }

    maint, err := h.service.GetMaintenance(ctx, id)
    if err != nil {
        return apierrors.HandleError(c, err, "get maintenance")
    }

    return c.JSON(http.StatusOK, maint)
}
```

## Echo Error Middleware

### Custom HTTP Error Handler

```go
package middlewares

import (
    "errors"
    "net/http"

    "github.com/labstack/echo/v4"
    "github.com/ruko1202/xlog"
    "go.uber.org/zap"

    "github.com/ruko1202/maintmode/internal/apperr"
    "github.com/ruko1202/maintmode/internal/app/api/apierrors"
)

func CustomHTTPErrorHandler(err error, c echo.Context) {
    ctx := c.Request().Context()

    // Extract Echo HTTP error if present
    var echoErr *echo.HTTPError
    if errors.As(err, &echoErr) {
        code := echoErr.Code
        message := echoErr.Message.(string)

        xlog.Warn(ctx, "HTTP error",
            zap.Int("status", code),
            zap.String("message", message),
        )

        _ = c.JSON(code, apierrors.NewErrorResponse(
            apierrors.ErrInvalidRequest,
            message,
        ))
        return
    }

    // Handle domain errors
    status := apierrors.DomainErrorToHTTPStatus(err)
    code := apierrors.DomainErrorToErrorCode(err)

    // Log appropriately
    if status >= 500 {
        xlog.Error(ctx, "server error", zap.Error(err))
    } else {
        xlog.Warn(ctx, "client error", zap.Error(err), zap.Int("status", status))
    }

    // Don't expose internal errors
    message := err.Error()
    if status == http.StatusInternalServerError {
        message = "internal server error"
    }

    _ = c.JSON(status, apierrors.NewErrorResponse(code, message))
}
```

**Register in Echo:**
```go
e := echo.New()
e.HTTPErrorHandler = middlewares.CustomHTTPErrorHandler
```

### Error Recovery Middleware

```go
package middlewares

import (
    "fmt"
    "net/http"
    "runtime/debug"

    "github.com/labstack/echo/v4"
    "github.com/ruko1202/xlog"
    "go.uber.org/zap"

    "github.com/ruko1202/maintmode/internal/app/api/apierrors"
)

func RecoverMiddleware() echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            defer func() {
                if r := recover(); r != nil {
                    ctx := c.Request().Context()
                    err := fmt.Errorf("panic: %v", r)

                    xlog.Error(ctx, "panic recovered",
                        zap.Error(err),
                        zap.String("stack", string(debug.Stack())),
                    )

                    _ = c.JSON(http.StatusInternalServerError, apierrors.NewErrorResponse(
                        apierrors.ErrInternalError,
                        "internal server error",
                    ))
                }
            }()

            return next(c)
        }
    }
}
```

## Error Response Formatting

### Standard Error Response

```go
package apierrors

type ErrorCode string

const (
    ErrNotFound       ErrorCode = "not_found"
    ErrInternalError  ErrorCode = "internal_error"
    ErrInvalidRequest ErrorCode = "invalid_request"
    ErrConflict       ErrorCode = "conflict"
    ErrForbidden      ErrorCode = "forbidden"
    ErrUnauthorized   ErrorCode = "unauthorized"
)

type ErrorResponse struct {
    Code    string `json:"code" example:"not_found"`
    Message string `json:"message" example:"resource not found"`
}

func NewErrorResponse(code ErrorCode, message string) *ErrorResponse {
    return &ErrorResponse{
        Code:    string(code),
        Message: message,
    }
}
```

### Validation Error Response

For detailed validation errors:

```go
package apierrors

type ValidationErrorResponse struct {
    Code    string            `json:"code"`
    Message string            `json:"message"`
    Fields  map[string]string `json:"fields,omitempty"`
}

func NewValidationErrorResponse(fields map[string]string) *ValidationErrorResponse {
    return &ValidationErrorResponse{
        Code:    string(ErrInvalidRequest),
        Message: "validation failed",
        Fields:  fields,
    }
}
```

**Usage:**
```go
func (h *Handler) CreateUser(c echo.Context) error {
    var req CreateUserRequest
    if err := c.Bind(&req); err != nil {
        return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
            apierrors.ErrInvalidRequest,
            "invalid request body",
        ))
    }

    // Validate
    validationErrs := make(map[string]string)
    if req.Email == "" {
        validationErrs["email"] = "email is required"
    }
    if req.Password == "" {
        validationErrs["password"] = "password is required"
    }

    if len(validationErrs) > 0 {
        return c.JSON(http.StatusUnprocessableEntity,
            apierrors.NewValidationErrorResponse(validationErrs))
    }

    // ...
}
```

## Client-Friendly Error Messages

### Message Guidelines

1. **Be specific but not technical**
   - Good: "User not found"
   - Bad: "sql: no rows in result set"

2. **Be actionable**
   - Good: "Email address is invalid. Please provide a valid email."
   - Bad: "Invalid input"

3. **Don't expose internals**
   - Good: "Failed to process request"
   - Bad: "Database connection failed: timeout after 30s on host db.internal:5432"

4. **Use proper grammar and punctuation**
   - Good: "Resource not found."
   - Bad: "resource not found"

### Message Conversion Pattern

```go
package apierrors

import "github.com/ruko1202/maintmode/internal/apperr"

func ToClientMessage(err error) string {
    switch {
    case errors.Is(err, apperr.ErrMaintNotFound):
        return "Maintenance window not found"
    case errors.Is(err, apperr.ErrResourceNotFound):
        return "Resource not found"
    case errors.Is(err, apperr.ErrInvalidPeriodEmptyStartOrEnd):
        return "Start and end times are required"
    case errors.Is(err, apperr.ErrInvalidPeriodStartOrEnd):
        return "Start time must be before end time"
    case errors.Is(err, apperr.ErrForbiddenStatusTransition):
        return "Cannot change maintenance status from current state"
    case errors.Is(err, apperr.ErrConflictsChangedSincePreview):
        return "Conflicts have changed. Please review and try again."
    default:
        return "An error occurred while processing your request"
    }
}
```

## Complete Handler Example

```go
package uicalendar

import (
    "errors"
    "net/http"

    "github.com/google/uuid"
    "github.com/labstack/echo/v4"
    "github.com/ruko1202/xlog"
    "go.uber.org/zap"

    "github.com/ruko1202/maintmode/internal/apperr"
    "github.com/ruko1202/maintmode/internal/app/api/apierrors"
)

func (i *Implementation) GetMaintenance(c echo.Context) error {
    ctx := xlog.WithOperation(c.Request().Context(), "api.Calendar.GetMaintenance")

    // Parse and validate input
    maintID, err := uuid.Parse(c.Param("id"))
    if err != nil {
        xlog.Warn(ctx, "invalid UUID format", zap.Error(err))
        return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
            apierrors.ErrInvalidRequest,
            "Maintenance ID must be a valid UUID",
        ))
    }

    // Call service
    maint, err := i.calendarSrv.GetMaintenance(ctx, maintID)
    if err != nil {
        // Log error
        xlog.Error(ctx, "get maintenance failed", zap.Error(err), zap.String("id", maintID.String()))

        // Map to HTTP response
        switch {
        case errors.Is(err, apperr.ErrMaintNotFound):
            return c.JSON(http.StatusNotFound, apierrors.NewErrorResponse(
                apierrors.ErrNotFound,
                "Maintenance window not found",
            ))
        case errors.Is(err, apperr.ErrForbidden):
            return c.JSON(http.StatusForbidden, apierrors.NewErrorResponse(
                apierrors.ErrForbidden,
                "Access to this maintenance window is forbidden",
            ))
        default:
            return c.JSON(http.StatusInternalServerError, apierrors.NewErrorResponse(
                apierrors.ErrInternalError,
                "Failed to retrieve maintenance window",
            ))
        }
    }

    return c.JSON(http.StatusOK, maint)
}
```

## Testing Error Responses

```go
package uicalendar_test

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/labstack/echo/v4"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/ruko1202/maintmode/internal/apperr"
    "github.com/ruko1202/maintmode/internal/app/api/apierrors"
)

func TestGetMaintenance_NotFound(t *testing.T) {
    // Setup
    service := &mockService{
        getMaintErr: apperr.ErrMaintNotFound,
    }
    handler := NewHandler(service)

    e := echo.New()
    req := httptest.NewRequest(http.MethodGet, "/maintenances/"+uuid.NewString(), nil)
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)

    // Execute
    err := handler.GetMaintenance(c)

    // Assert
    require.NoError(t, err)
    assert.Equal(t, http.StatusNotFound, rec.Code)

    var response apierrors.ErrorResponse
    require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
    assert.Equal(t, string(apierrors.ErrNotFound), response.Code)
    assert.Contains(t, response.Message, "not found")
}

func TestGetMaintenance_InternalError(t *testing.T) {
    // Setup
    service := &mockService{
        getMaintErr: errors.New("database error"),
    }
    handler := NewHandler(service)

    e := echo.New()
    req := httptest.NewRequest(http.MethodGet, "/maintenances/"+uuid.NewString(), nil)
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)

    // Execute
    err := handler.GetMaintenance(c)

    // Assert
    require.NoError(t, err)
    assert.Equal(t, http.StatusInternalServerError, rec.Code)

    var response apierrors.ErrorResponse
    require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
    assert.Equal(t, string(apierrors.ErrInternalError), response.Code)
}
```

## Best Practices Summary

1. **Centralize error mapping** in a helper function
2. **Use consistent error response format**
3. **Log errors appropriately** (error vs warn based on status)
4. **Don't expose internal details** in 5xx responses
5. **Provide actionable messages** in 4xx responses
6. **Use proper HTTP status codes**
7. **Include error codes** for programmatic handling
8. **Test error scenarios** thoroughly
9. **Document error codes** in API documentation
10. **Consider validation error details** for better UX
