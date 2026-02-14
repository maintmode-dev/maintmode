---
name: testify-testing
description: Comprehensive guide for writing unit and integration tests in Go using testify framework. Use this skill when writing tests, creating test suites, mocking dependencies, testing HTTP handlers (Echo), testing services with business logic, testing database operations (PostgreSQL with jet/sqlx), implementing table-driven tests, choosing between require and assert assertions, organizing test files, or improving test coverage. Essential for MaintMode project's clean architecture testing (app/services/storages layers).
---

# Testify Testing

## Overview

Write effective unit and integration tests using testify for Go projects, with specific patterns for the MaintMode clean architecture stack (Echo v4, PostgreSQL, Jet, sqlx).

## Quick Start

```go
import (
    "testing"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/suite"
)

func TestBasicFunction(t *testing.T) {
    result := Add(2, 3)
    require.Equal(t, 5, result)
}
```

## Core Testing Patterns

### 1. Table-Driven Tests

Use table-driven tests for testing multiple scenarios with the same logic. Ideal for testing functions with various inputs and expected outputs.

**When to use:**
- Testing multiple input/output combinations
- Testing edge cases and boundary conditions
- Testing validation logic
- Testing error scenarios

**Basic example:**

```go
func TestValidateEmail(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name    string
        email   string
        wantErr bool
    }{
        {"valid email", "user@example.com", false},
        {"invalid format", "not-an-email", true},
        {"empty", "", true},
    }

    for _, tt := range tests {
        tt := tt // Capture range variable for parallel tests
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            err := ValidateEmail(tt.email)
            if tt.wantErr {
                require.Error(t, err)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

**For comprehensive patterns and advanced usage:** See [references/table-driven-tests.md](references/table-driven-tests.md)

### 2. Assertions: require vs assert

**Default to `require`** - it stops test execution on failure, providing clear failure points.

```go
user, err := GetUser(id)
require.NoError(t, err)        // Stop if error
require.NotNil(t, user)        // Stop if nil
require.Equal(t, "John", user.Name) // Safe to access
```

**Use `assert` rarely** - only when checking multiple independent conditions:

```go
// Check all validation errors at once
assert.False(t, user.IsValidEmail())
assert.False(t, user.IsValidAge())
assert.False(t, user.IsValidName())
```

**Common assertions:**
- `require.NoError(t, err)` - most important, check errors first
- `require.Equal(t, expected, actual)` - equality check
- `require.NotNil(t, value)` - nil check before accessing
- `require.Contains(t, slice, item)` - collection membership
- `require.Len(t, slice, n)` - length check
- `require.ErrorContains(t, err, "text")` - partial error message match

**For complete assertion reference:** See [references/assertions.md](references/assertions.md)

### 3. Test Suites

Test suites group related tests and share setup/teardown logic. Best for integration tests with databases or external dependencies.

**When to use:**
- Tests need shared database connections
- Tests need common setup/teardown
- Testing integration with external services
- Testing multiple methods of the same component

**Basic suite:**

```go
type MaintenanceStorageSuite struct {
    suite.Suite
    db    *sqlx.DB
    store *maintenances.Store
}

func (s *MaintenanceStorageSuite) SetupSuite() {
    // Connect once for all tests
    var err error
    s.db, err = sqlx.Connect("postgres", os.Getenv("TEST_DB_URL"))
    s.Require().NoError(err)
}

func (s *MaintenanceStorageSuite) SetupTest() {
    // Clean database before each test
    s.store = maintenances.NewStore(s.db)
    _, err := s.db.Exec("TRUNCATE maintenances CASCADE")
    s.Require().NoError(err)
}

func (s *MaintenanceStorageSuite) TearDownSuite() {
    s.db.Close()
}

func (s *MaintenanceStorageSuite) TestCreate() {
    ctx := context.Background()
    maint := &entity.Maintenance{
        ID:     uuid.New(),
        Title:  "Test",
        Status: entity.StatusPlanned,
    }

    err := s.store.Create(ctx, maint)
    s.Require().NoError(err)

    result, err := s.store.Get(ctx, maint.ID)
    s.Require().NoError(err)
    s.Equal(maint.ID, result.ID)
}

func TestMaintenanceStorageSuite(t *testing.T) {
    suite.Run(t, new(MaintenanceStorageSuite))
}
```

**For complete suite patterns:** See [references/test-suites.md](references/test-suites.md)

### 4. Mocking with testify/mock

Mock external dependencies like storage, APIs, and services. Use mockery to generate mocks from interfaces.

**Generate mocks:**

```bash
# Install mockery
go install github.com/vektra/mockery/v2@latest

# Generate mock for an interface
mockery --name=MaintenanceStorage --output=./mocks --outpkg=mocks
```

**Use mocks in tests:**

```go
func TestMaintenanceService_Create(t *testing.T) {
    mockStorage := mocks.NewMaintenanceStorage(t)
    mockNotifier := mocks.NewNotifier(t)
    service := services.NewMaintenanceService(mockStorage, mockNotifier)

    ctx := context.Background()
    maint := &entity.Maintenance{Title: "Test", Status: entity.StatusPlanned}

    // Setup expectations
    mockStorage.On("Create", ctx, mock.MatchedBy(func(m *entity.Maintenance) bool {
        return m.ID != uuid.Nil && m.Title == "Test"
    })).Return(nil)

    mockNotifier.On("NotifyMaintenancePlanned", ctx, mock.Anything).Return(nil)

    // Execute
    result, err := service.Create(ctx, maint)

    // Assert
    require.NoError(t, err)
    require.NotNil(t, result)
    mockStorage.AssertExpectations(t)
    mockNotifier.AssertExpectations(t)
}
```

**For complete mocking guide:** See [references/mocking.md](references/mocking.md)

## MaintMode-Specific Testing Patterns

### Testing Echo HTTP Handlers

```go
func TestMaintenanceHandler_Create(t *testing.T) {
    e := echo.New()
    mockService := mocks.NewMaintenanceService(t)
    handler := NewMaintenanceHandler(mockService)

    reqBody := `{"title":"Test","status":"planned"}`
    req := httptest.NewRequest(http.MethodPost, "/api/maintenances", strings.NewReader(reqBody))
    req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)

    expected := &entity.Maintenance{ID: uuid.New(), Title: "Test"}
    mockService.On("Create", mock.Anything, mock.Anything).Return(expected, nil)

    err := handler.Create(c)

    require.NoError(t, err)
    require.Equal(t, http.StatusCreated, rec.Code)
    mockService.AssertExpectations(t)
}
```

### Testing Service Layer

```go
func TestMaintenanceService_Create(t *testing.T) {
    mockStorage := mocks.NewMaintenanceStorage(t)
    service := services.NewMaintenanceService(mockStorage)

    ctx := context.Background()
    input := &dto.CreateMaintenanceInput{
        Title:  "Test Maintenance",
        Status: entity.StatusPlanned,
    }

    mockStorage.On("Create", ctx, mock.MatchedBy(func(m *entity.Maintenance) bool {
        return m.ID != uuid.Nil &&
               m.Title == input.Title &&
               !m.CreatedAt.IsZero()
    })).Return(nil)

    result, err := service.Create(ctx, input)

    require.NoError(t, err)
    require.NotNil(t, result)
    require.Equal(t, input.Title, result.Title)
    mockStorage.AssertExpectations(t)
}
```

### Testing Storage Layer with PostgreSQL

```go
func TestMaintenanceStorage_Create(t *testing.T) {
    t.Parallel()

    ctx := context.Background()
    store := maintenances.NewStore(db)

    maint := &entity.Maintenance{
        ID:     uuid.New(),
        Title:  "Test",
        PlannedPeriod: entity.NewPeriod(
            time.Now().Add(time.Hour),
            time.Now().Add(2*time.Hour),
        ),
        Status:    entity.StatusPlanned,
        CreatedAt: time.Now().UTC(),
    }

    err := store.Create(ctx, maint)
    require.NoError(t, err)

    result, err := store.Get(ctx, maint.ID)
    require.NoError(t, err)
    require.Equal(t, maint.ID, result.ID)
}
```

**For comprehensive MaintMode testing patterns:** See [references/testing-patterns.md](references/testing-patterns.md)

## Test Organization

### File Structure

```
internal/
├── app/                      # Handler layer
│   └── maintenances/
│       ├── handler.go
│       └── handler_test.go
├── services/                 # Business logic layer
│   └── maintenances/
│       ├── service.go
│       └── service_test.go
└── storages/                # Data access layer
    └── maintenances/
        ├── storage.go
        ├── create_test.go    # One test file per operation
        ├── get_test.go
        └── list_test.go
```

### Test Naming Conventions

- **Functions:** `TestFunctionName_Scenario`
- **Methods:** `TestType_Method_Scenario`

```go
func TestValidateEmail_ValidInput(t *testing.T) {}
func TestMaintenanceService_Create_Success(t *testing.T) {}
func TestMaintenanceService_Create_ValidationError(t *testing.T) {}
```

## Testing Decision Tree

Use this flow to choose the right testing approach:

```
Start: Need to write test
│
├─ Testing database operations?
│  └─ YES → Use test suite + real database
│     - SetupSuite: connect to DB
│     - SetupTest: clean tables
│     - Use t.Parallel() for independent tests
│
├─ Testing HTTP handlers?
│  └─ YES → Mock service layer + httptest
│     - Create echo.Echo instance
│     - Mock service dependencies
│     - Use httptest.NewRequest/NewRecorder
│
├─ Testing service/business logic?
│  └─ YES → Mock storage/dependencies
│     - Generate mocks with mockery
│     - Setup expectations with mock.On()
│     - Assert with AssertExpectations()
│
├─ Testing multiple scenarios?
│  └─ YES → Use table-driven tests
│     - Define test cases as slice of structs
│     - Use t.Run() for subtests
│     - Add t.Parallel() if tests are independent
│
└─ Simple function test?
   └─ YES → Basic test with require
      - Setup input
      - Call function
      - Assert with require
```

## Coverage Guidelines

### Run Coverage

```bash
# Basic coverage
go test -cover ./...

# Detailed coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Coverage by package
go test -cover ./internal/services/...
go test -cover ./internal/storages/...
```

### Coverage Targets

- **Services (business logic):** 80%+ coverage
- **Handlers (HTTP layer):** 60%+ coverage
- **Storage (data access):** 70%+ coverage
- **Critical paths:** 90%+ coverage

### What to Test

**High priority:**
- Business logic and validation
- Error handling
- Critical user flows
- Data integrity operations

**Lower priority:**
- Generated code (Jet models, mocks)
- Simple getters/setters
- Logging statements

## Best Practices

1. **Default to `require`** - Use `require` for most assertions, `assert` rarely
2. **Check errors first** - Always `require.NoError()` before using return values
3. **Check nil before access** - Always `require.NotNil()` before accessing fields
4. **Use parallel tests** - Add `t.Parallel()` when tests are independent
5. **Use subtests** - Always use `t.Run()` for better organization
6. **Clean state** - Reset database/mocks between tests
7. **Use table-driven tests** - For multiple scenarios with same logic
8. **Name tests clearly** - Test names should describe the scenario
9. **Mock at boundaries** - Mock storage and external services, test real business logic
10. **Generate mocks** - Use mockery, don't write mocks by hand

## Common Patterns Summary

| Pattern | Use Case | Key Points |
|---------|----------|------------|
| Table-driven | Multiple scenarios | Use `t.Run()`, `t.Parallel()` |
| Test suites | Shared setup/DB tests | `SetupSuite`, `SetupTest`, `TearDown` |
| Mocking | External dependencies | `mockery`, `mock.On()`, `AssertExpectations()` |
| Handler tests | HTTP endpoints | `httptest`, mock services |
| Service tests | Business logic | Mock storage, test logic |
| Storage tests | Database operations | Real DB, test suites, `t.Parallel()` |

## Reference Files

For detailed information on specific topics:

- **[table-driven-tests.md](references/table-driven-tests.md)** - Complete guide to table-driven test patterns, parallel execution, complex test cases
- **[assertions.md](references/assertions.md)** - Full assertion reference, require vs assert, error handling, MaintMode patterns
- **[test-suites.md](references/test-suites.md)** - Suite lifecycle, integration tests, service/handler suite examples
- **[mocking.md](references/mocking.md)** - Mock generation, expectations, argument matchers, advanced patterns
- **[testing-patterns.md](references/testing-patterns.md)** - Echo handlers, services, storage, PostgreSQL/Jet, coverage strategies

## Quick Reference

### Import Packages

```go
import (
    "testing"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/suite"
)
```

### Essential Commands

```bash
# Run tests
go test ./...

# Run with coverage
go test -cover ./...

# Run parallel tests
go test -parallel 8 ./...

# Run specific test
go test -run TestFunctionName ./...

# Generate mocks
mockery --name=InterfaceName --output=./mocks
```

### Key Assertion Methods

```go
require.NoError(t, err)
require.Equal(t, expected, actual)
require.NotNil(t, value)
require.Contains(t, slice, item)
require.Len(t, slice, n)
require.True(t, condition)
require.ErrorContains(t, err, "text")
```
