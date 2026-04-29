# Testing Transactional Code

Comprehensive guide for testing services and stores that use transactions in the MaintMode project.

## Table of Contents

- [Testing Philosophy](#testing-philosophy)
- [Test Environment Setup](#test-environment-setup)
- [Testing Service Layer with Transactions](#testing-service-layer-with-transactions)
- [Testing Store Layer](#testing-store-layer)
- [Transaction Rollback Testing](#transaction-rollback-testing)
- [Integration Tests with Real Transactions](#integration-tests-with-real-transactions)
- [Test Isolation Strategies](#test-isolation-strategies)
- [Common Testing Patterns](#common-testing-patterns)
- [Best Practices](#best-practices)

## Testing Philosophy

When testing transactional code:

1. **Test with real transactions** - Use actual database transactions in integration tests
2. **Test both success and failure paths** - Verify commits and rollbacks
3. **Isolate tests** - Each test should be independent and parallel-safe
4. **Test transaction boundaries** - Verify operations are atomic
5. **Test concurrent scenarios** - Validate locking and isolation behavior

## Test Environment Setup

### TestMain Setup

Create a `main_test.go` file to initialize test dependencies:

```go
package test

import (
    "context"
    "os"
    "testing"

    "github.com/jmoiron/sqlx"
    "github.com/ruko1202/xlog"
    "go.uber.org/zap"

    "github.com/ruko1202/maintmode/internal/services/maint"
    "github.com/ruko1202/maintmode/internal/storages/maintenances"
    "github.com/ruko1202/maintmode/internal/storages/resources"
    "github.com/ruko1202/maintmode/internal/utils/dbtx"
    "github.com/ruko1202/maintmode/internal/utils/closer"
    testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
    db         *sqlx.DB
    maintStore *maintenances.Store
)

func TestMain(m *testing.M) {
    ctx := context.Background()
    logger, _ := zap.NewDevelopment()
    xlog.ReplaceGlobal(logger)

    // Initialize test database connection
    conn := testdbconnutils.NewDB()
    closer.Add(conn.Close)
    db = conn

    // Initialize stores
    maintStore = maintenances.NewStore(conn)

    code := m.Run()

    closer.CloseAll(ctx)
    os.Exit(code)
}

func initService(db *sqlx.DB) *maint.Service {
    return maint.NewService(
        dbtx.NewTxManager(db),
        maintStore,
        resources.NewStore(db),
        conflictsSrv,
    )
}
```

**Key points:**
- Share database connection across tests
- Initialize stores once in TestMain
- Clean up resources after all tests complete
- Create service instances per test for isolation

## Testing Service Layer with Transactions

### Basic Service Test with Transaction

Test services that use `WithinTx`:

```go
func TestCreateDraft(t *testing.T) {
    t.Parallel()

    ctx := context.Background()
    now := xtime.UTCNow()
    s := initService(db)

    t.Run("ok", func(t *testing.T) {
        t.Parallel()

        cmd := &entity.CreateMaintenanceCmd{
            Title:         "Title" + t.Name(),
            Description:   "Description" + t.Name(),
            PlannedPeriod: entity.NewPeriod(now, now.Add(time.Hour)),
            Impact:        entity.MaintenanceImpactFull,
            Resources: []*entity.Resource{{
                ID:   xuuid.New(),
                Type: entity.ResourceTypeService,
            }},
        }

        // Call service method that uses transaction
        maint, err := s.CreateDraft(ctx, cmd)
        require.NoError(t, err)
        require.NotNil(t, maint)
        require.NotEmpty(t, maint.ID)

        // Verify all data was committed
        require.Equal(t, cmd.Title, maint.Title)
        require.Equal(t, cmd.Description, maint.Description)
        require.Equal(t, cmd.PlannedPeriod, maint.PlannedPeriod)
        require.Equal(t, cmd.Impact, maint.Impact)
    })
}
```

**Key points:**
- Use `t.Parallel()` for concurrent test execution
- Generate unique test data using `t.Name()`
- Verify returned data matches input
- Transaction is automatically committed on success

### Testing Transaction Rollback on Error

Test that errors trigger rollback:

```go
func TestCreateDraft_RollbackOnError(t *testing.T) {
    t.Parallel()

    ctx := context.Background()
    s := initService(db)

    t.Run("invalid input causes rollback", func(t *testing.T) {
        t.Parallel()

        cmd := &entity.CreateMaintenanceCmd{
            Title:         "", // Invalid: empty title
            PlannedPeriod: entity.NewPeriod(now, now.Add(time.Hour)),
        }

        // Service returns error
        maint, err := s.CreateDraft(ctx, cmd)
        require.Error(t, err)
        require.Nil(t, maint)

        // Verify nothing was committed
        if maint != nil {
            dbMaint, err := s.Get(ctx, maint.ID)
            require.Error(t, err)
            require.Nil(t, dbMaint)
        }
    })
}
```

**Key points:**
- Test with invalid input to trigger errors
- Verify method returns error
- Confirm no data was persisted
- Transaction automatically rolls back

### Testing Multiple Operations in Transaction

Test that all operations are atomic:

```go
func TestUpdateWithResources(t *testing.T) {
    t.Parallel()

    ctx := context.Background()
    s := initService(db)

    t.Run("all operations succeed or none", func(t *testing.T) {
        t.Parallel()

        // Create initial maintenance
        maint := createTestMaintenance(t, s)

        // Update with additional resources
        cmd := &entity.UpdateMaintenanceCmd{
            Title:       "Updated Title",
            Description: "Updated Description",
            Resources: []*entity.Resource{{
                ID:   xuuid.New(),
                Type: entity.ResourceTypeService,
            }},
        }

        err := s.Update(ctx, maint.ID, cmd)
        require.NoError(t, err)

        // Verify all changes were applied atomically
        updated, err := s.Get(ctx, maint.ID)
        require.NoError(t, err)
        require.Equal(t, cmd.Title, updated.Title)
        require.Equal(t, cmd.Description, updated.Description)
        require.Len(t, updated.Resources, 1)
    })
}
```

**Key points:**
- Test compound operations
- Verify all-or-nothing atomicity
- Check all changes were applied together

## Testing Store Layer

### Basic Store Test

Test store methods without explicit transaction handling:

```go
func TestCreate(t *testing.T) {
    t.Parallel()

    ctx := context.Background()
    now := xtime.UTCNow()
    store := NewStore(db)

    t.Run("ok", func(t *testing.T) {
        t.Parallel()

        period := entity.NewPeriod(now.Add(-time.Hour), now.Add(time.Hour))
        maint := &entity.Maintenance{
            ID:            xuuid.New(),
            Title:         "Title" + t.Name(),
            Description:   "Description" + t.Name(),
            PlannedPeriod: period,
            Status:        entity.MaintenanceStatusPlanned,
            Impact:        entity.MaintenanceImpactFull,
            CreatedAt:     now,
        }

        err := store.Create(ctx, maint)
        require.NoError(t, err)

        // Verify data was persisted
        dbMaint, err := store.Get(ctx, maint.ID)
        require.NoError(t, err)
        require.Equal(t, maint, dbMaint)
    })
}
```

**Key points:**
- Store tests use direct database connection
- No explicit transaction management
- Tests are self-contained and isolated

### Testing Store with Manual Transaction

Test store behavior within a transaction context:

```go
func TestStoreWithTransaction(t *testing.T) {
    t.Parallel()

    ctx := context.Background()
    txManager := dbtx.NewTxManager(db)
    store := NewStore(db)

    t.Run("operations use transaction from context", func(t *testing.T) {
        t.Parallel()

        var createdID uuid.UUID

        err := txManager.WithinTx(ctx, func(ctx context.Context) error {
            maint := buildTestMaintenance(t)

            if err := store.Create(ctx, maint); err != nil {
                return err
            }

            createdID = maint.ID

            // Can read within same transaction
            dbMaint, err := store.Get(ctx, maint.ID)
            if err != nil {
                return err
            }
            require.Equal(t, maint.Title, dbMaint.Title)

            return nil
        })

        require.NoError(t, err)

        // Verify committed after transaction completes
        dbMaint, err := store.Get(ctx, createdID)
        require.NoError(t, err)
        require.NotNil(t, dbMaint)
    })
}
```

**Key points:**
- Create transaction explicitly with TxManager
- Store methods automatically use transaction from context
- Can read data within same transaction
- Data persists after successful commit

## Transaction Rollback Testing

### Testing Automatic Rollback on Error

```go
func TestRollbackOnError(t *testing.T) {
    t.Parallel()

    ctx := context.Background()
    txManager := dbtx.NewTxManager(db)
    store := NewStore(db)

    t.Run("rollback on validation error", func(t *testing.T) {
        t.Parallel()

        var maintID uuid.UUID

        err := txManager.WithinTx(ctx, func(ctx context.Context) error {
            maint := buildTestMaintenance(t)
            maintID = maint.ID

            if err := store.Create(ctx, maint); err != nil {
                return err
            }

            // Simulate error after partial work
            return fmt.Errorf("validation failed")
        })

        require.Error(t, err)
        require.Contains(t, err.Error(), "validation failed")

        // Verify data was rolled back
        dbMaint, err := store.Get(ctx, maintID)
        require.Error(t, err)
        require.Nil(t, dbMaint)
    })
}
```

### Testing Rollback with Multiple Operations

```go
func TestRollbackMultipleOperations(t *testing.T) {
    t.Parallel()

    ctx := context.Background()
    txManager := dbtx.NewTxManager(db)
    maintStore := maintenances.NewStore(db)
    resourceStore := resources.NewStore(db)

    t.Run("all operations rolled back on error", func(t *testing.T) {
        t.Parallel()

        var maintID, resourceID uuid.UUID

        err := txManager.WithinTx(ctx, func(ctx context.Context) error {
            // Create maintenance
            maint := buildTestMaintenance(t)
            maintID = maint.ID
            if err := maintStore.Create(ctx, maint); err != nil {
                return err
            }

            // Create resource
            resource := buildTestResource(t)
            resourceID = resource.ID
            if err := resourceStore.Create(ctx, resource); err != nil {
                return err
            }

            // Simulate error
            return fmt.Errorf("business logic error")
        })

        require.Error(t, err)

        // Verify both operations were rolled back
        _, err = maintStore.Get(ctx, maintID)
        require.Error(t, err)

        _, err = resourceStore.Get(ctx, resourceID)
        require.Error(t, err)
    })
}
```

**Key points:**
- Explicitly trigger errors to test rollback
- Verify no partial data persists
- Test with multiple operations
- All-or-nothing atomicity guaranteed

## Integration Tests with Real Transactions

### Testing Concurrent Transactions

Test transaction isolation and locking:

```go
func TestConcurrentUpdates(t *testing.T) {
    t.Parallel()

    ctx := context.Background()
    s := initService(db)

    // Create initial maintenance
    maint := createTestMaintenance(t, s)

    // Run concurrent updates
    var wg sync.WaitGroup
    errors := make(chan error, 2)

    for i := 0; i < 2; i++ {
        wg.Add(1)
        go func(index int) {
            defer wg.Done()

            err := s.Update(ctx, maint.ID, &entity.UpdateMaintenanceCmd{
                Title: fmt.Sprintf("Update %d", index),
            })
            errors <- err
        }(i)
    }

    wg.Wait()
    close(errors)

    // Both should succeed (one after the other)
    for err := range errors {
        require.NoError(t, err)
    }

    // Verify final state is consistent
    final, err := s.Get(ctx, maint.ID)
    require.NoError(t, err)
    require.NotEmpty(t, final.Title)
}
```

### Testing FOR UPDATE Locking

Test pessimistic locking behavior:

```go
func TestForUpdateLocking(t *testing.T) {
    ctx := context.Background()
    txManager := dbtx.NewTxManager(db)
    store := NewStore(db)

    // Create test record
    maint := createTestMaintenance(t, store)

    var wg sync.WaitGroup
    results := make(chan string, 2)

    // First transaction - holds lock
    wg.Add(1)
    go func() {
        defer wg.Done()

        err := txManager.WithinTx(ctx, func(ctx context.Context) error {
            // Acquire lock
            m, err := store.GetForUpdate(ctx, maint.ID)
            if err != nil {
                return err
            }

            results <- "tx1-acquired"
            time.Sleep(100 * time.Millisecond)

            m.Title = "Updated by TX1"
            return store.Update(ctx, m)
        })
        require.NoError(t, err)
        results <- "tx1-committed"
    }()

    // Second transaction - waits for lock
    time.Sleep(10 * time.Millisecond) // Ensure TX1 starts first
    wg.Add(1)
    go func() {
        defer wg.Done()

        err := txManager.WithinTx(ctx, func(ctx context.Context) error {
            results <- "tx2-waiting"

            // This will wait for TX1 to commit
            m, err := store.GetForUpdate(ctx, maint.ID)
            if err != nil {
                return err
            }

            results <- "tx2-acquired"
            m.Title = "Updated by TX2"
            return store.Update(ctx, m)
        })
        require.NoError(t, err)
        results <- "tx2-committed"
    }()

    wg.Wait()
    close(results)

    // Verify order: TX1 acquires, TX2 waits, TX1 commits, TX2 acquires
    events := []string{}
    for r := range results {
        events = append(events, r)
    }

    require.Equal(t, "tx1-acquired", events[0])
    require.Equal(t, "tx2-waiting", events[1])
    require.Equal(t, "tx1-committed", events[2])
    require.Equal(t, "tx2-acquired", events[3])
    require.Equal(t, "tx2-committed", events[4])

    // Final state should reflect TX2's update
    final, err := store.Get(ctx, maint.ID)
    require.NoError(t, err)
    require.Equal(t, "Updated by TX2", final.Title)
}
```

**Key points:**
- Use goroutines to simulate concurrent access
- Test lock acquisition order
- Verify pessimistic locking prevents conflicts
- Confirm final state is consistent

## Test Isolation Strategies

### Strategy 1: Unique Test Data

Generate unique data per test to avoid conflicts:

```go
func TestIsolation_UniqueData(t *testing.T) {
    t.Parallel()

    ctx := context.Background()
    s := initService(db)

    t.Run("test 1", func(t *testing.T) {
        t.Parallel()

        // Use t.Name() for uniqueness
        cmd := &entity.CreateMaintenanceCmd{
            Title: "Title" + t.Name(),
            // ... other fields
        }

        maint, err := s.CreateDraft(ctx, cmd)
        require.NoError(t, err)
        // ... assertions
    })

    t.Run("test 2", func(t *testing.T) {
        t.Parallel()

        // Different unique data
        cmd := &entity.CreateMaintenanceCmd{
            Title: "Title" + t.Name(),
            // ... other fields
        }

        maint, err := s.CreateDraft(ctx, cmd)
        require.NoError(t, err)
        // ... assertions
    })
}
```

**Benefits:**
- Tests can run in parallel
- No cleanup required
- Fast execution
- Simple to implement

**Drawbacks:**
- Database grows with test data
- May need periodic cleanup

### Strategy 2: Test Transactions with Rollback

Wrap entire test in transaction that rolls back:

```go
func TestIsolation_TestTransaction(t *testing.T) {
    ctx := context.Background()

    // Begin test transaction
    tx, err := db.BeginTxx(ctx, nil)
    require.NoError(t, err)
    defer tx.Rollback()

    // Use transaction context for all operations
    txCtx := dbtx.WithTx(ctx, tx)

    store := NewStore(db)

    // Run test operations
    maint := buildTestMaintenance(t)
    err = store.Create(txCtx, maint)
    require.NoError(t, err)

    // Verify within transaction
    dbMaint, err := store.Get(txCtx, maint.ID)
    require.NoError(t, err)
    require.Equal(t, maint.Title, dbMaint.Title)

    // Transaction rolls back automatically
}
```

**Benefits:**
- Complete test isolation
- No cleanup needed
- No leftover test data

**Drawbacks:**
- Cannot test actual commit behavior
- Cannot run tests in parallel
- Nested transactions not supported

### Strategy 3: Cleanup Functions

Use cleanup functions to remove test data:

```go
func TestIsolation_Cleanup(t *testing.T) {
    t.Parallel()

    ctx := context.Background()
    s := initService(db)

    maint := createTestMaintenance(t, s)

    // Register cleanup
    t.Cleanup(func() {
        _ = s.Delete(ctx, maint.ID)
    })

    // Run test
    err := s.Update(ctx, maint.ID, &entity.UpdateMaintenanceCmd{
        Title: "Updated",
    })
    require.NoError(t, err)

    // Verify
    updated, err := s.Get(ctx, maint.ID)
    require.NoError(t, err)
    require.Equal(t, "Updated", updated.Title)

    // Cleanup runs automatically
}
```

**Benefits:**
- Clean database after tests
- Can run in parallel
- Tests actual commit/rollback

**Drawbacks:**
- More complex setup
- Cleanup must handle errors
- May leave orphaned data if test panics

## Common Testing Patterns

### Pattern 1: Table-Driven Tests for Transactions

```go
func TestCreateWithValidation(t *testing.T) {
    t.Parallel()

    ctx := context.Background()
    s := initService(db)
    now := xtime.UTCNow()

    tests := []struct {
        name        string
        cmd         *entity.CreateMaintenanceCmd
        wantErr     bool
        errContains string
    }{
        {
            name: "valid maintenance",
            cmd: &entity.CreateMaintenanceCmd{
                Title:         "Valid Title",
                Description:   "Valid Description",
                PlannedPeriod: entity.NewPeriod(now, now.Add(time.Hour)),
                Impact:        entity.MaintenanceImpactFull,
            },
            wantErr: false,
        },
        {
            name: "empty title",
            cmd: &entity.CreateMaintenanceCmd{
                Title:         "",
                Description:   "Valid Description",
                PlannedPeriod: entity.NewPeriod(now, now.Add(time.Hour)),
            },
            wantErr:     true,
            errContains: "title is required",
        },
        {
            name: "invalid period",
            cmd: &entity.CreateMaintenanceCmd{
                Title:         "Valid Title",
                Description:   "Valid Description",
                PlannedPeriod: entity.NewPeriod(now.Add(time.Hour), now),
            },
            wantErr:     true,
            errContains: "period",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            maint, err := s.CreateDraft(ctx, tt.cmd)

            if tt.wantErr {
                require.Error(t, err)
                if tt.errContains != "" {
                    require.Contains(t, err.Error(), tt.errContains)
                }
                require.Nil(t, maint)
            } else {
                require.NoError(t, err)
                require.NotNil(t, maint)
            }
        })
    }
}
```

### Pattern 2: Helper Functions for Test Data

```go
// Test helpers
func createTestMaintenance(t *testing.T, s *Service) *entity.Maintenance {
    t.Helper()

    ctx := context.Background()
    now := xtime.UTCNow()

    cmd := &entity.CreateMaintenanceCmd{
        Title:         "Test Maintenance " + t.Name(),
        Description:   "Test Description",
        PlannedPeriod: entity.NewPeriod(now, now.Add(time.Hour)),
        Impact:        entity.MaintenanceImpactFull,
    }

    maint, err := s.CreateDraft(ctx, cmd)
    require.NoError(t, err)
    return maint
}

func buildTestMaintenance(t *testing.T) *entity.Maintenance {
    t.Helper()

    now := xtime.UTCNow()
    return &entity.Maintenance{
        ID:            xuuid.New(),
        Title:         "Test " + t.Name(),
        Description:   "Description",
        PlannedPeriod: entity.NewPeriod(now, now.Add(time.Hour)),
        Status:        entity.MaintenanceStatusDraft,
        Impact:        entity.MaintenanceImpactFull,
        CreatedAt:     now,
    }
}
```

### Pattern 3: Testing Error Propagation

```go
func TestErrorPropagation(t *testing.T) {
    t.Parallel()

    ctx := context.Background()
    s := initService(db)

    t.Run("store error propagates to service", func(t *testing.T) {
        t.Parallel()

        // Create with duplicate ID to trigger store error
        maint1 := createTestMaintenance(t, s)

        // Try to create again with same data
        cmd := &entity.CreateMaintenanceCmd{
            Title:         maint1.Title,
            Description:   maint1.Description,
            PlannedPeriod: maint1.PlannedPeriod,
            Impact:        maint1.Impact,
        }

        // Should fail at service level
        _, err := s.CreateDraft(ctx, cmd)

        // Error should be wrapped and propagated
        require.Error(t, err)

        // Transaction should have rolled back
        // No duplicate data exists
    })
}
```

## Best Practices

### 1. Always Use t.Parallel()

```go
func TestSomething(t *testing.T) {
    t.Parallel() // Enable parallel execution

    t.Run("subtest", func(t *testing.T) {
        t.Parallel() // Enable parallel subtests
        // test code
    })
}
```

### 2. Generate Unique Test Data

```go
// Good: unique per test
Title: "Test " + t.Name()

// Bad: hardcoded, may conflict
Title: "Test Maintenance"
```

### 3. Use Helper Functions

```go
func createTestMaintenance(t *testing.T, s *Service) *entity.Maintenance {
    t.Helper() // Mark as helper for better error reporting
    // ... creation logic
}
```

### 4. Test Both Success and Failure

```go
func TestOperation(t *testing.T) {
    t.Run("success", func(t *testing.T) {
        // Test happy path
    })

    t.Run("failure - validation error", func(t *testing.T) {
        // Test error handling
    })

    t.Run("failure - database error", func(t *testing.T) {
        // Test rollback
    })
}
```

### 5. Verify Transaction Boundaries

```go
// Good: verify data persists after transaction
err := s.Update(ctx, id, cmd)
require.NoError(t, err)

// Read back to confirm commit
result, err := s.Get(ctx, id)
require.NoError(t, err)
require.Equal(t, cmd.Title, result.Title)
```

### 6. Use require for Critical Assertions

```go
// Good: stops test immediately on failure
require.NoError(t, err)
require.NotNil(t, result)

// Bad: continues executing after failure
assert.NoError(t, err)
assert.NotNil(t, result) // may panic if result is nil
```

### 7. Test Concurrent Access

```go
func TestConcurrentAccess(t *testing.T) {
    var wg sync.WaitGroup
    errors := make(chan error, numGoroutines)

    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            err := performOperation()
            errors <- err
        }()
    }

    wg.Wait()
    close(errors)

    // Verify all succeeded
    for err := range errors {
        require.NoError(t, err)
    }
}
```

### 8. Clean Up Resources

```go
func TestWithCleanup(t *testing.T) {
    resource := createResource(t)

    t.Cleanup(func() {
        cleanupResource(resource)
    })

    // Use resource in test
}
```

## Summary

Effective testing of transactional code requires:

1. **Real database integration tests** - Test actual transaction behavior
2. **Isolation strategies** - Keep tests independent and parallel-safe
3. **Both paths tested** - Success commits, failures rollback
4. **Concurrent scenarios** - Verify locking and isolation
5. **Helper functions** - Reduce duplication, improve readability
6. **Proper assertions** - Use require for critical checks
7. **Cleanup strategies** - Manage test data lifecycle

Follow these patterns to build a robust test suite for transactional services in the MaintMode project.
