# Testing Patterns for MaintMode

This document covers common testing patterns specific to the MaintMode project's architecture and technology stack.

## Table of Contents

- [Testing Echo HTTP Handlers](#testing-echo-http-handlers)
- [Testing Service Layer](#testing-service-layer)
- [Testing Storage Layer with Database](#testing-storage-layer-with-database)
- [Testing with PostgreSQL and Jet](#testing-with-postgresql-and-jet)
- [Test Organization](#test-organization)
- [Coverage Strategies](#coverage-strategies)

## Testing Echo HTTP Handlers

### Basic Handler Test

```go
func TestMaintenanceHandler_Get(t *testing.T) {
    e := echo.New()
    mockService := mocks.NewMaintenanceService(t)
    handler := NewMaintenanceHandler(mockService)

    maintID := uuid.New()
    expected := &entity.Maintenance{
        ID:     maintID,
        Title:  "Test Maintenance",
        Status: entity.StatusPlanned,
    }

    mockService.On("Get", mock.Anything, maintID).Return(expected, nil)

    // Create request
    req := httptest.NewRequest(http.MethodGet, "/api/maintenances/"+maintID.String(), nil)
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)
    c.SetParamNames("id")
    c.SetParamValues(maintID.String())

    // Execute
    err := handler.Get(c)

    // Assert
    require.NoError(t, err)
    require.Equal(t, http.StatusOK, rec.Code)

    var result entity.Maintenance
    err = json.Unmarshal(rec.Body.Bytes(), &result)
    require.NoError(t, err)
    require.Equal(t, expected.ID, result.ID)
    require.Equal(t, expected.Title, result.Title)

    mockService.AssertExpectations(t)
}
```

### Handler Test with Request Body

```go
func TestMaintenanceHandler_Create(t *testing.T) {
    e := echo.New()
    mockService := mocks.NewMaintenanceService(t)
    handler := NewMaintenanceHandler(mockService)

    reqBody := `{
        "title": "Scheduled Maintenance",
        "description": "System upgrade",
        "status": "planned"
    }`

    expected := &entity.Maintenance{
        ID:          uuid.New(),
        Title:       "Scheduled Maintenance",
        Description: "System upgrade",
        Status:      entity.StatusPlanned,
    }

    mockService.On("Create", mock.Anything, mock.MatchedBy(func(m *entity.Maintenance) bool {
        return m.Title == "Scheduled Maintenance" && m.Status == entity.StatusPlanned
    })).Return(expected, nil)

    req := httptest.NewRequest(http.MethodPost, "/api/maintenances", strings.NewReader(reqBody))
    req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)

    err := handler.Create(c)

    require.NoError(t, err)
    require.Equal(t, http.StatusCreated, rec.Code)
    mockService.AssertExpectations(t)
}
```

### Handler Error Response Test

```go
func TestMaintenanceHandler_Get_NotFound(t *testing.T) {
    e := echo.New()
    mockService := mocks.NewMaintenanceService(t)
    handler := NewMaintenanceHandler(mockService)

    maintID := uuid.New()
    mockService.On("Get", mock.Anything, maintID).Return(nil, apperr.ErrMaintNotFound)

    req := httptest.NewRequest(http.MethodGet, "/api/maintenances/"+maintID.String(), nil)
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)
    c.SetParamNames("id")
    c.SetParamValues(maintID.String())

    err := handler.Get(c)

    require.Error(t, err)

    // Check if it's an Echo HTTP error
    var httpErr *echo.HTTPError
    require.ErrorAs(t, err, &httpErr)
    require.Equal(t, http.StatusNotFound, httpErr.Code)

    mockService.AssertExpectations(t)
}
```

### Handler with Query Parameters

```go
func TestMaintenanceHandler_List(t *testing.T) {
    e := echo.New()
    mockService := mocks.NewMaintenanceService(t)
    handler := NewMaintenanceHandler(mockService)

    expected := []*entity.Maintenance{
        {ID: uuid.New(), Title: "Maint 1"},
        {ID: uuid.New(), Title: "Maint 2"},
    }

    mockService.On("List", mock.Anything, mock.MatchedBy(func(f *dto.Filter) bool {
        return f.Status == entity.StatusPlanned && f.Limit == 10
    })).Return(expected, false, nil)

    req := httptest.NewRequest(http.MethodGet, "/api/maintenances?status=planned&limit=10", nil)
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)

    err := handler.List(c)

    require.NoError(t, err)
    require.Equal(t, http.StatusOK, rec.Code)

    var result []*entity.Maintenance
    err = json.Unmarshal(rec.Body.Bytes(), &result)
    require.NoError(t, err)
    require.Len(t, result, 2)

    mockService.AssertExpectations(t)
}
```

## Testing Service Layer

### Service Test with Multiple Dependencies

```go
func TestMaintenanceService_Create(t *testing.T) {
    mockStorage := mocks.NewMaintenanceStorage(t)
    mockNotifier := mocks.NewNotifier(t)
    mockValidator := mocks.NewValidator(t)

    service := services.NewMaintenanceService(
        mockStorage,
        mockNotifier,
        mockValidator,
    )

    ctx := context.Background()
    now := time.Now().UTC()

    input := &dto.CreateMaintenanceInput{
        Title:       "Test",
        Description: "Description",
        Status:      entity.StatusPlanned,
    }

    // Setup expectations
    mockValidator.On("ValidateCreate", input).Return(nil)

    mockStorage.On("Create", ctx, mock.MatchedBy(func(m *entity.Maintenance) bool {
        return m.ID != uuid.Nil &&
               m.Title == input.Title &&
               m.CreatedAt.After(now.Add(-time.Second))
    })).Return(nil)

    mockNotifier.On("NotifyMaintenancePlanned", ctx, mock.Anything).Return(nil)

    // Execute
    result, err := service.Create(ctx, input)

    // Assert
    require.NoError(t, err)
    require.NotNil(t, result)
    require.NotEqual(t, uuid.Nil, result.ID)
    require.Equal(t, input.Title, result.Title)
    require.False(t, result.CreatedAt.IsZero())

    mockStorage.AssertExpectations(t)
    mockNotifier.AssertExpectations(t)
    mockValidator.AssertExpectations(t)
}
```

### Service Error Handling Test

```go
func TestMaintenanceService_Create_ValidationError(t *testing.T) {
    mockStorage := mocks.NewMaintenanceStorage(t)
    mockNotifier := mocks.NewNotifier(t)
    mockValidator := mocks.NewValidator(t)

    service := services.NewMaintenanceService(mockStorage, mockNotifier, mockValidator)

    ctx := context.Background()
    input := &dto.CreateMaintenanceInput{Title: ""} // Invalid

    expectedErr := apperr.ErrValidation
    mockValidator.On("ValidateCreate", input).Return(expectedErr)

    result, err := service.Create(ctx, input)

    require.Error(t, err)
    require.Nil(t, result)
    require.ErrorIs(t, err, expectedErr)

    // Storage and Notifier should not be called
    mockStorage.AssertNotCalled(t, "Create")
    mockNotifier.AssertNotCalled(t, "NotifyMaintenancePlanned")
}
```

### Service Business Logic Test

```go
func TestMaintenanceService_StartMaintenance(t *testing.T) {
    tests := []struct {
        name          string
        currentStatus entity.MaintenanceStatus
        wantErr       bool
        expectedErr   error
    }{
        {
            name:          "start from planned",
            currentStatus: entity.StatusPlanned,
            wantErr:       false,
        },
        {
            name:          "cannot start from completed",
            currentStatus: entity.StatusCompleted,
            wantErr:       true,
            expectedErr:   apperr.ErrInvalidStatusTransition,
        },
        {
            name:          "cannot start from cancelled",
            currentStatus: entity.StatusCancelled,
            wantErr:       true,
            expectedErr:   apperr.ErrInvalidStatusTransition,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockStorage := mocks.NewMaintenanceStorage(t)
            service := services.NewMaintenanceService(mockStorage, nil, nil)

            ctx := context.Background()
            maintID := uuid.New()

            existing := &entity.Maintenance{
                ID:     maintID,
                Status: tt.currentStatus,
            }

            mockStorage.On("Get", ctx, maintID).Return(existing, nil)

            if !tt.wantErr {
                mockStorage.On("Update", ctx, mock.MatchedBy(func(m *entity.Maintenance) bool {
                    return m.Status == entity.StatusInProgress &&
                           m.ActualPeriod != nil
                })).Return(nil)
            }

            err := service.StartMaintenance(ctx, maintID)

            if tt.wantErr {
                require.Error(t, err)
                require.ErrorIs(t, err, tt.expectedErr)
            } else {
                require.NoError(t, err)
            }

            mockStorage.AssertExpectations(t)
        })
    }
}
```

## Testing Storage Layer with Database

### Integration Test Setup

```go
var db *sqlx.DB

func TestMain(m *testing.M) {
    var err error

    // Connect to test database
    db, err = sqlx.Connect("postgres", os.Getenv("TEST_DB_URL"))
    if err != nil {
        log.Fatal("failed to connect to test db:", err)
    }
    defer db.Close()

    // Run migrations
    if err := runMigrations(db); err != nil {
        log.Fatal("failed to run migrations:", err)
    }

    // Run tests
    code := m.Run()

    os.Exit(code)
}
```

### Storage Create Test

```go
func TestMaintenanceStorage_Create(t *testing.T) {
    t.Parallel()

    ctx := context.Background()
    now := time.Now().UTC()
    store := maintenances.NewStore(db)

    maint := &entity.Maintenance{
        ID:          uuid.New(),
        Title:       "Test Maintenance",
        Description: "Test Description",
        PlannedPeriod: entity.NewPeriod(
            now.Add(time.Hour),
            now.Add(2*time.Hour),
        ),
        Status:    entity.StatusPlanned,
        Impact:    entity.ImpactFull,
        CreatedAt: now,
    }

    err := store.Create(ctx, maint)
    require.NoError(t, err)

    // Verify by reading back
    result, err := store.Get(ctx, maint.ID)
    require.NoError(t, err)
    require.Equal(t, maint.ID, result.ID)
    require.Equal(t, maint.Title, result.Title)
    require.Equal(t, maint.Status, result.Status)
}
```

### Storage Error Test

```go
func TestMaintenanceStorage_Create_InvalidPeriod(t *testing.T) {
    t.Parallel()

    ctx := context.Background()
    now := time.Now().UTC()
    store := maintenances.NewStore(db)

    tests := []struct {
        name        string
        period      entity.Period
        expectedErr string
    }{
        {
            name:        "start equals end",
            period:      entity.NewPeriod(now, now),
            expectedErr: "violates check constraint",
        },
        {
            name:        "start after end",
            period:      entity.NewPeriod(now.Add(time.Hour), now),
            expectedErr: "range lower bound must be less than or equal to range upper bound",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            maint := &entity.Maintenance{
                ID:            uuid.New(),
                Title:         "Test",
                PlannedPeriod: tt.period,
                Status:        entity.StatusPlanned,
                Impact:        entity.ImpactFull,
                CreatedAt:     now,
            }

            err := store.Create(ctx, maint)
            require.Error(t, err)
            require.ErrorContains(t, err, tt.expectedErr)
        })
    }
}
```

### Storage List/Query Test

```go
func TestMaintenanceStorage_List(t *testing.T) {
    t.Parallel()

    ctx := context.Background()
    store := maintenances.NewStore(db)
    now := time.Now().UTC()

    // Create test data
    maint1 := createTestMaintenance(ctx, t, store, now, entity.StatusPlanned)
    maint2 := createTestMaintenance(ctx, t, store, now, entity.StatusInProgress)
    maint3 := createTestMaintenance(ctx, t, store, now, entity.StatusPlanned)

    t.Run("filter by status", func(t *testing.T) {
        filter := &dto.Filter{Status: entity.StatusPlanned}
        results, truncated, err := store.List(ctx, filter, 100)

        require.NoError(t, err)
        require.False(t, truncated)
        require.GreaterOrEqual(t, len(results), 2)

        // Verify results contain our test data
        ids := lo.Map(results, func(m *entity.Maintenance, _ int) uuid.UUID {
            return m.ID
        })
        require.Contains(t, ids, maint1.ID)
        require.Contains(t, ids, maint3.ID)
        require.NotContains(t, ids, maint2.ID)
    })

    t.Run("limit and truncation", func(t *testing.T) {
        results, truncated, err := store.List(ctx, &dto.Filter{}, 1)

        require.NoError(t, err)
        require.True(t, truncated)
        require.Len(t, results, 1)
    })
}

func createTestMaintenance(
    ctx context.Context,
    t *testing.T,
    store *maintenances.Store,
    now time.Time,
    status entity.MaintenanceStatus,
) *entity.Maintenance {
    t.Helper()

    maint := &entity.Maintenance{
        ID:     uuid.New(),
        Title:  "Test " + uuid.New().String(),
        Status: status,
        PlannedPeriod: entity.NewPeriod(
            now.Add(time.Hour),
            now.Add(2*time.Hour),
        ),
        Impact:    entity.ImpactPartial,
        CreatedAt: now,
    }

    err := store.Create(ctx, maint)
    require.NoError(t, err)

    return maint
}
```

## Testing with PostgreSQL and Jet

### Jet Query Test

```go
func TestMaintenanceStorage_GetWithResources(t *testing.T) {
    t.Parallel()

    ctx := context.Background()
    store := maintenances.NewStore(db)

    // Create maintenance with resources
    maint := createTestMaintenance(ctx, t, store, time.Now().UTC(), entity.StatusPlanned)
    resource1 := createTestResource(ctx, t, db, "Resource 1")
    resource2 := createTestResource(ctx, t, db, "Resource 2")

    err := store.AddResources(ctx, maint.ID, []uuid.UUID{resource1.ID, resource2.ID})
    require.NoError(t, err)

    // Query with Jet
    result, err := store.GetWithResources(ctx, maint.ID)
    require.NoError(t, err)
    require.NotNil(t, result)
    require.Len(t, result.Resources, 2)

    resourceIDs := lo.Map(result.Resources, func(r *entity.Resource, _ int) uuid.UUID {
        return r.ID
    })
    require.Contains(t, resourceIDs, resource1.ID)
    require.Contains(t, resourceIDs, resource2.ID)
}
```

## Test Organization

### File Structure

```
internal/
├── app/                    # Application/handler layer
│   └── maintenances/
│       ├── handler.go
│       └── handler_test.go
├── services/              # Business logic layer
│   └── maintenances/
│       ├── service.go
│       └── service_test.go
└── storages/             # Data access layer
    └── maintenances/
        ├── storage.go
        ├── create_test.go
        ├── get_test.go
        ├── update_test.go
        └── list_test.go
```

### Test Naming Conventions

```go
// Function: TestFunctionName_Scenario
func TestCreate_ValidInput(t *testing.T) {}
func TestCreate_InvalidEmail(t *testing.T) {}

// Method: TestType_Method_Scenario
func TestMaintenanceService_Create_Success(t *testing.T) {}
func TestMaintenanceService_Create_ValidationError(t *testing.T) {}
```

## Coverage Strategies

### Unit Test Coverage

Focus on:
- Business logic in services
- Validation logic
- Error handling
- Edge cases

```bash
# Run tests with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Integration Test Coverage

Focus on:
- Database operations
- Complex queries
- Transaction handling
- Data consistency

```bash
# Run only integration tests
go test -tags=integration ./...

# Run integration tests with database
TEST_DB_URL=postgres://... go test ./internal/storages/...
```

### Table-Driven Test Coverage

Ensure test cases cover:
- Happy path
- Error conditions
- Boundary values
- Edge cases
- Invalid input

```go
tests := []struct {
    name string
    // ... test fields
}{
    {"happy path", ...},
    {"empty input", ...},
    {"null value", ...},
    {"boundary max", ...},
    {"boundary min", ...},
    {"invalid format", ...},
}
```

### Coverage Best Practices

1. **Aim for 80%+ coverage in services** - Business logic should be well-tested
2. **Lower coverage for handlers** - Focus on integration tests
3. **High coverage for critical paths** - Payment, security, data integrity
4. **Don't test generated code** - Mock interfaces, Jet models
5. **Focus on behavior, not lines** - Coverage is a metric, not a goal
