# Test Suites

Test suites in testify provide a way to group related tests and share setup/teardown logic across multiple test methods.

## Basic Suite Structure

```go
package mypackage

import (
    "testing"
    "github.com/stretchr/testify/suite"
)

// Suite definition
type UserServiceSuite struct {
    suite.Suite
    service *UserService
    db      *sql.DB
}

// Setup - runs once before all tests
func (s *UserServiceSuite) SetupSuite() {
    var err error
    s.db, err = sql.Open("postgres", "connection-string")
    s.Require().NoError(err)
}

// TearDown - runs once after all tests
func (s *UserServiceSuite) TearDownSuite() {
    s.db.Close()
}

// SetupTest - runs before each test
func (s *UserServiceSuite) SetupTest() {
    s.service = NewUserService(s.db)
}

// TearDownTest - runs after each test
func (s *UserServiceSuite) TearDownTest() {
    // Clean up after each test if needed
}

// Test methods - must start with "Test"
func (s *UserServiceSuite) TestCreateUser() {
    user, err := s.service.Create("john@example.com")
    s.Require().NoError(err)
    s.Require().NotNil(user)
    s.Equal("john@example.com", user.Email)
}

func (s *UserServiceSuite) TestGetUser() {
    // Setup
    user, _ := s.service.Create("jane@example.com")

    // Test
    found, err := s.service.Get(user.ID)
    s.Require().NoError(err)
    s.Equal(user.ID, found.ID)
}

// Run the suite
func TestUserServiceSuite(t *testing.T) {
    suite.Run(t, new(UserServiceSuite))
}
```

## Suite Lifecycle Hooks

```go
type MySuite struct {
    suite.Suite
}

// SetupSuite - runs once before all tests in the suite
func (s *MySuite) SetupSuite() {
    // Initialize shared resources
    // Example: database connection, external service setup
}

// TearDownSuite - runs once after all tests in the suite
func (s *MySuite) TearDownSuite() {
    // Cleanup shared resources
    // Example: close database connection
}

// SetupTest - runs before each test method
func (s *MySuite) SetupTest() {
    // Reset state before each test
    // Example: clear database tables, reset mocks
}

// TearDownTest - runs after each test method
func (s *MySuite) TearDownTest() {
    // Cleanup after each test
}

// BeforeTest - runs before each test, receives test info
func (s *MySuite) BeforeTest(suiteName, testName string) {
    // Can use test name for conditional setup
}

// AfterTest - runs after each test, receives test info
func (s *MySuite) AfterTest(suiteName, testName string) {
    // Can use test name for conditional cleanup
}
```

## Integration Test Suite Pattern

Ideal for tests that need database connections or external dependencies:

```go
type MaintenanceStorageSuite struct {
    suite.Suite
    db    *sqlx.DB
    store *maintenances.Store
}

func (s *MaintenanceStorageSuite) SetupSuite() {
    // Connect to test database
    var err error
    s.db, err = sqlx.Connect("postgres", os.Getenv("TEST_DB_URL"))
    s.Require().NoError(err)

    // Run migrations
    err = runMigrations(s.db)
    s.Require().NoError(err)
}

func (s *MaintenanceStorageSuite) TearDownSuite() {
    s.db.Close()
}

func (s *MaintenanceStorageSuite) SetupTest() {
    // Initialize store for each test
    s.store = maintenances.NewStore(s.db)

    // Clean database tables
    _, err := s.db.Exec("TRUNCATE maintenances CASCADE")
    s.Require().NoError(err)
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
    s.Equal(maint, result)
}

func (s *MaintenanceStorageSuite) TestUpdate() {
    // Test implementation
}

func TestMaintenanceStorageSuite(t *testing.T) {
    suite.Run(t, new(MaintenanceStorageSuite))
}
```

## Suite with Subtests

Combine suites with table-driven tests:

```go
func (s *ValidationSuite) TestEmailValidation() {
    tests := []struct {
        name    string
        email   string
        wantErr bool
    }{
        {"valid email", "user@example.com", false},
        {"invalid format", "not-an-email", true},
        {"empty email", "", true},
    }

    for _, tt := range tests {
        s.Run(tt.name, func() {
            err := s.validator.ValidateEmail(tt.email)
            if tt.wantErr {
                s.Error(err)
            } else {
                s.NoError(err)
            }
        })
    }
}
```

## Accessing Suite Methods

Suite embeds `*testing.T`, providing direct access to assertion methods:

```go
func (s *MySuite) TestExample() {
    // These are equivalent:
    s.Equal(expected, actual)        // Suite method
    s.Require().Equal(expected, actual) // Require (stops on failure)
    s.Assert().Equal(expected, actual)  // Assert (continues on failure)

    // Access underlying *testing.T
    s.T().Helper()
    s.T().Parallel()
}
```

## Service Layer Test Suite Example

```go
type UserServiceSuite struct {
    suite.Suite
    service      *UserService
    mockStorage  *mocks.UserStorage
    mockNotifier *mocks.Notifier
}

func (s *UserServiceSuite) SetupTest() {
    s.mockStorage = mocks.NewUserStorage(s.T())
    s.mockNotifier = mocks.NewNotifier(s.T())
    s.service = NewUserService(s.mockStorage, s.mockNotifier)
}

func (s *UserServiceSuite) TestCreateUser_Success() {
    email := "test@example.com"
    expectedUser := &User{ID: "123", Email: email}

    s.mockStorage.On("Create", mock.Anything, email).Return(expectedUser, nil)
    s.mockNotifier.On("SendWelcome", email).Return(nil)

    user, err := s.service.CreateUser(context.Background(), email)

    s.Require().NoError(err)
    s.Equal(expectedUser, user)
    s.mockStorage.AssertExpectations(s.T())
    s.mockNotifier.AssertExpectations(s.T())
}

func (s *UserServiceSuite) TestCreateUser_StorageError() {
    email := "test@example.com"
    expectedErr := errors.New("database error")

    s.mockStorage.On("Create", mock.Anything, email).Return(nil, expectedErr)

    user, err := s.service.CreateUser(context.Background(), email)

    s.Require().Error(err)
    s.Nil(user)
    s.Equal(expectedErr, err)
    s.mockStorage.AssertExpectations(s.T())
    // Notifier should not be called
    s.mockNotifier.AssertNotCalled(s.T(), "SendWelcome")
}

func TestUserServiceSuite(t *testing.T) {
    suite.Run(t, new(UserServiceSuite))
}
```

## HTTP Handler Test Suite Example

```go
type HandlerSuite struct {
    suite.Suite
    echo    *echo.Echo
    handler *MaintenanceHandler
    mockSvc *mocks.MaintenanceService
}

func (s *HandlerSuite) SetupTest() {
    s.echo = echo.New()
    s.mockSvc = mocks.NewMaintenanceService(s.T())
    s.handler = NewMaintenanceHandler(s.mockSvc)
}

func (s *HandlerSuite) TestCreateMaintenance() {
    reqBody := `{"title":"Test","status":"planned"}`
    req := httptest.NewRequest(http.MethodPost, "/maintenances", strings.NewReader(reqBody))
    req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
    rec := httptest.NewRecorder()
    c := s.echo.NewContext(req, rec)

    expected := &entity.Maintenance{ID: uuid.New(), Title: "Test"}
    s.mockSvc.On("Create", mock.Anything, mock.Anything).Return(expected, nil)

    err := s.handler.Create(c)

    s.Require().NoError(err)
    s.Equal(http.StatusCreated, rec.Code)
    s.mockSvc.AssertExpectations(s.T())
}

func TestHandlerSuite(t *testing.T) {
    suite.Run(t, new(HandlerSuite))
}
```

## When to Use Suites

**Use suites when:**
- Tests need shared setup/teardown logic
- Testing integration with databases or external services
- Multiple tests operate on the same resource
- Need lifecycle hooks (before/after each test)

**Don't use suites when:**
- Tests are simple and independent
- Setup is minimal or test-specific
- Parallel execution is important (suites run sequentially by default)
- Table-driven tests are sufficient

## Best Practices

1. **Keep suite scope focused** - Group related functionality
2. **Clean state between tests** - Use SetupTest/TearDownTest
3. **Use Require() for critical assertions** - Stop test on failure
4. **Name tests clearly** - Method names should start with "Test"
5. **Minimize shared state** - Prefer test-specific setup when possible
6. **Document dependencies** - Comment on required resources
7. **Don't overuse** - Simple tests don't need suites
8. **Use mocks in SetupTest** - Recreate mocks for each test to avoid interference

## Suite vs Regular Tests

```go
// Regular test - good for simple, independent tests
func TestSimpleFunction(t *testing.T) {
    result := Add(2, 3)
    require.Equal(t, 5, result)
}

// Suite - good for tests with shared setup
type CalculatorSuite struct {
    suite.Suite
    calc *Calculator
}

func (s *CalculatorSuite) SetupTest() {
    s.calc = NewCalculator()
}

func (s *CalculatorSuite) TestAdd() {
    s.Equal(5, s.calc.Add(2, 3))
}

func (s *CalculatorSuite) TestMultiply() {
    s.Equal(6, s.calc.Multiply(2, 3))
}

func TestCalculatorSuite(t *testing.T) {
    suite.Run(t, new(CalculatorSuite))
}
```
