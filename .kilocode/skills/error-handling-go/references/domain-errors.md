# Domain Error Patterns

## Error Package Organization

### Single Error Package (Recommended for Small-Medium Projects)

```
internal/
└── apperr/
    ├── errors.go          # Common errors
    ├── validation.go      # Validation errors
    └── business.go        # Business logic errors
```

**Example: errors.go**
```go
package apperr

import "errors"

// Common errors
var (
    ErrNotFound      = errors.New("resource not found")
    ErrAlreadyExists = errors.New("resource already exists")
    ErrConflict      = errors.New("resource conflict")
    ErrUnauthorized  = errors.New("unauthorized")
    ErrForbidden     = errors.New("forbidden access")
    ErrInvalidInput  = errors.New("invalid input")
)
```

### Domain-Specific Error Packages (Recommended for Large Projects)

```
internal/
├── apperr/
│   └── errors.go          # Shared common errors
├── user/
│   └── errors.go          # User domain errors
├── order/
│   └── errors.go          # Order domain errors
└── payment/
    └── errors.go          # Payment domain errors
```

**Example: user/errors.go**
```go
package user

import (
    "errors"
    "fmt"
)

var (
    ErrUserNotFound       = errors.New("user not found")
    ErrUserAlreadyExists  = errors.New("user already exists")
    ErrInvalidEmail       = errors.New("invalid email")
    ErrWeakPassword       = errors.New("weak password")
    ErrEmailNotVerified   = errors.New("email not verified")
)

// Parameterized errors
func DuplicateEmail(email string) error {
    return fmt.Errorf("%w: %s", ErrUserAlreadyExists, email)
}
```

## Error Hierarchies

### Using Error Wrapping

Build error hierarchies using `fmt.Errorf` with `%w`:

```go
package apperr

import (
    "errors"
    "fmt"
)

// Base errors
var (
    ErrValidation = errors.New("validation failed")
    ErrNotFound   = errors.New("not found")
)

// Specific validation errors
var (
    ErrInvalidEmail    = fmt.Errorf("%w: invalid email format", ErrValidation)
    ErrInvalidPassword = fmt.Errorf("%w: password too weak", ErrValidation)
    ErrInvalidAge      = fmt.Errorf("%w: age must be positive", ErrValidation)
)

// Specific not found errors
var (
    ErrUserNotFound    = fmt.Errorf("%w: user", ErrNotFound)
    ErrProductNotFound = fmt.Errorf("%w: product", ErrNotFound)
    ErrOrderNotFound   = fmt.Errorf("%w: order", ErrNotFound)
)
```

**Usage:**
```go
// Check for specific error
if errors.Is(err, apperr.ErrUserNotFound) {
    // Handle user not found
}

// Check for base error category
if errors.Is(err, apperr.ErrNotFound) {
    // Handle any not found error
}

// Check for validation category
if errors.Is(err, apperr.ErrValidation) {
    // Handle any validation error
}
```

### Using Custom Error Types

For complex error hierarchies with structured data:

```go
package apperr

import "fmt"

// Base validation error type
type ValidationError struct {
    Field   string
    Value   interface{}
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
}

// Specific validation errors
type EmailValidationError struct {
    ValidationError
}

type PasswordValidationError struct {
    ValidationError
    MinLength int
}

func (e *PasswordValidationError) Error() string {
    return fmt.Sprintf("password too weak: minimum %d characters required", e.MinLength)
}

// Constructor functions
func NewEmailValidationError(email string) error {
    return &EmailValidationError{
        ValidationError: ValidationError{
            Field:   "email",
            Value:   email,
            Message: "invalid email format",
        },
    }
}

func NewPasswordValidationError(password string, minLength int) error {
    return &PasswordValidationError{
        ValidationError: ValidationError{
            Field:   "password",
            Value:   password,
            Message: "password too weak",
        },
        MinLength: minLength,
    }
}
```

**Usage:**
```go
// Check for any validation error
var validationErr *apperr.ValidationError
if errors.As(err, &validationErr) {
    log.Printf("Validation failed: field=%s, message=%s",
        validationErr.Field, validationErr.Message)
}

// Check for specific validation error
var passwordErr *apperr.PasswordValidationError
if errors.As(err, &passwordErr) {
    log.Printf("Password validation failed: min length=%d", passwordErr.MinLength)
}
```

## Error Wrapping Strategies

### Strategy 1: Wrap at Every Layer

**Pros**: Full error trace
**Cons**: Verbose error messages

```go
// Storage layer
func (r *Repo) Get(ctx context.Context, id uuid.UUID) (*User, error) {
    user, err := r.db.QueryRow(ctx, query, id)
    if err != nil {
        return nil, fmt.Errorf("repository.Get: %w", err)
    }
    return user, nil
}

// Service layer
func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (*User, error) {
    user, err := s.repo.Get(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("service.GetUser: %w", err)
    }
    return user, nil
}

// Result: "service.GetUser: repository.Get: sql: no rows in result set"
```

### Strategy 2: Wrap with Context Only When Needed

**Pros**: Clean error messages
**Cons**: May lose some context

```go
// Storage layer
func (r *Repo) Get(ctx context.Context, id uuid.UUID) (*User, error) {
    user, err := r.db.QueryRow(ctx, query, id)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, apperr.ErrUserNotFound
        }
        return nil, fmt.Errorf("query user by id: %w", err)
    }
    return user, nil
}

// Service layer
func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (*User, error) {
    user, err := s.repo.Get(ctx, id)
    if err != nil {
        // Don't wrap known domain errors
        if errors.Is(err, apperr.ErrUserNotFound) {
            return nil, err
        }
        // Wrap unexpected errors
        return nil, fmt.Errorf("get user: %w", err)
    }
    return user, nil
}
```

### Strategy 3: Preserve Sentinel Errors

**Always preserve sentinel errors when wrapping:**

```go
// Good: Preserves sentinel error
func (s *Service) CreateUser(ctx context.Context, email string) error {
    exists, err := s.repo.EmailExists(ctx, email)
    if err != nil {
        return fmt.Errorf("check email exists: %w", err)
    }
    if exists {
        return apperr.ErrUserAlreadyExists  // Sentinel error, don't wrap
    }
    // ...
}

// Also good: Adds context while preserving sentinel
func (s *Service) CreateUser(ctx context.Context, email string) error {
    exists, err := s.repo.EmailExists(ctx, email)
    if err != nil {
        return fmt.Errorf("check email exists: %w", err)
    }
    if exists {
        return fmt.Errorf("%w: %s", apperr.ErrUserAlreadyExists, email)
    }
    // ...
}
```

## MaintMode Project Examples

Based on your existing code:

### Current Pattern (Good)

```go
// internal/apperr/errors.go
var (
    ErrMaintNotFound                = errors.New("maintenance not found")
    ErrResourceNotFound             = errors.New("resource not found")
    ErrInvalidPeriodEmptyStartOrEnd = errors.New("invalid period: empty start or end")
    ErrInvalidPeriodStartOrEnd      = errors.New("invalid period: start > end or start == end")
    ErrInvalidPeriodInterval        = errors.New("invalid period interval")
    ErrForbiddenStatusTransition    = errors.New("forbidden status maintenance")
    ErrConflictsChangedSincePreview = errors.New("conflicts changed since preview")
    ErrMaintChangedSincePreview     = errors.New("maintenance changed since preview")
)

func ForbiddenStatusTransition(currentStatus any) error {
    return fmt.Errorf("%w: %v", ErrForbiddenStatusTransition, currentStatus)
}
```

### Recommended Enhancement

Add error categories for better organization:

```go
package apperr

import (
    "errors"
    "fmt"
)

// Error categories
var (
    ErrNotFound   = errors.New("not found")
    ErrValidation = errors.New("validation error")
    ErrConflict   = errors.New("conflict error")
    ErrForbidden  = errors.New("forbidden")
)

// Specific not found errors
var (
    ErrMaintNotFound    = fmt.Errorf("%w: maintenance", ErrNotFound)
    ErrResourceNotFound = fmt.Errorf("%w: resource", ErrNotFound)
)

// Specific validation errors
var (
    ErrInvalidPeriodEmptyStartOrEnd = fmt.Errorf("%w: empty start or end", ErrValidation)
    ErrInvalidPeriodStartOrEnd      = fmt.Errorf("%w: start > end or start == end", ErrValidation)
    ErrInvalidPeriodInterval        = fmt.Errorf("%w: invalid interval", ErrValidation)
)

// Specific conflict errors
var (
    ErrConflictsChangedSincePreview = fmt.Errorf("%w: conflicts changed since preview", ErrConflict)
    ErrMaintChangedSincePreview     = fmt.Errorf("%w: maintenance changed since preview", ErrConflict)
)

// Specific forbidden errors
var (
    ErrForbiddenStatusTransition = fmt.Errorf("%w: status transition", ErrForbidden)
)

// Parameterized errors
func ForbiddenStatusTransition(currentStatus any) error {
    return fmt.Errorf("%w: %v", ErrForbiddenStatusTransition, currentStatus)
}
```

**Benefits:**
```go
// Can check for specific error
if errors.Is(err, apperr.ErrMaintNotFound) {
    return http.StatusNotFound
}

// Can check for error category
if errors.Is(err, apperr.ErrNotFound) {
    return http.StatusNotFound  // Handles both ErrMaintNotFound and ErrResourceNotFound
}

if errors.Is(err, apperr.ErrValidation) {
    return http.StatusUnprocessableEntity  // Handles all validation errors
}
```

## Multi-Error Pattern

For operations that can have multiple errors (e.g., batch operations, validation):

```go
package apperr

import (
    "errors"
    "fmt"
    "strings"
)

type MultiError struct {
    Errors []error
}

func (m *MultiError) Error() string {
    var msgs []string
    for _, err := range m.Errors {
        msgs = append(msgs, err.Error())
    }
    return strings.Join(msgs, "; ")
}

func (m *MultiError) Add(err error) {
    if err != nil {
        m.Errors = append(m.Errors, err)
    }
}

func (m *MultiError) HasErrors() bool {
    return len(m.Errors) > 0
}

func (m *MultiError) Is(target error) bool {
    for _, err := range m.Errors {
        if errors.Is(err, target) {
            return true
        }
    }
    return false
}
```

**Usage:**
```go
func (s *Service) ValidateUser(user *User) error {
    var errs MultiError

    if user.Email == "" {
        errs.Add(apperr.ErrInvalidEmail)
    }
    if user.Age < 0 {
        errs.Add(apperr.ErrInvalidAge)
    }
    if len(user.Password) < 8 {
        errs.Add(apperr.ErrWeakPassword)
    }

    if errs.HasErrors() {
        return &errs
    }
    return nil
}
```

## Best Practices Summary

1. **Use sentinel errors** for common, well-known errors
2. **Use error wrapping** to build error hierarchies
3. **Preserve sentinel errors** when adding context
4. **Use custom error types** for structured error data
5. **Organize errors** by domain for large projects
6. **Don't wrap domain errors** unnecessarily in service layer
7. **Always wrap** unexpected/infrastructure errors
8. **Use descriptive error messages** that help debugging
9. **Test error conditions** with `errors.Is()` and `errors.As()`
10. **Document error categories** for API consumers
