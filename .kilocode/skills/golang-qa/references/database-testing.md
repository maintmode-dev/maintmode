# Database Testing

Integration testing with real databases using testcontainers and testing patterns.

## Integration Tests with Testcontainers

```go
import (
    "context"
    "testing"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
)

func TestUserRepository(t *testing.T) {
    ctx := context.Background()

    // Start PostgreSQL container
    req := testcontainers.ContainerRequest{
        Image:        "postgres:16-alpine",
        ExposedPorts: []string{"5432/tcp"},
        Env: map[string]string{
            "POSTGRES_USER":     "test",
            "POSTGRES_PASSWORD": "test",
            "POSTGRES_DB":       "testdb",
        },
        WaitingFor: wait.ForLog("database system is ready to accept connections"),
    }

    postgres, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
    require.NoError(t, err)
    defer postgres.Terminate(ctx)

    // Get connection string
    host, _ := postgres.Host(ctx)
    port, _ := postgres.MappedPort(ctx, "5432")
    dsn := fmt.Sprintf("postgres://test:test@%s:%s/testdb?sslmode=disable", host, port.Port())

    // Connect and run tests
    db, err := sql.Open("postgres", dsn)
    require.NoError(t, err)
    defer db.Close()

    // Run migrations
    err = runMigrations(db)
    require.NoError(t, err)

    // Test repository
    repo := NewUserRepository(db)
    user := &User{Name: "John", Email: "john@example.com"}
    err = repo.SaveUser(user)
    assert.NoError(t, err)
    assert.NotZero(t, user.ID)
}
```

## Testing with Transactions

```go
func TestUserRepositoryWithTransaction(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    tests := []struct {
        name string
        test func(*testing.T, *sql.DB)
    }{
        {"create user", testCreateUser},
        {"update user", testUpdateUser},
        {"delete user", testDeleteUser},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Start transaction for test isolation
            tx, err := db.Begin()
            require.NoError(t, err)
            defer tx.Rollback() // Always rollback

            // Run test with transaction
            tt.test(t, tx)
        })
    }
}

func testCreateUser(t *testing.T, db *sql.DB) {
    repo := NewUserRepository(db)
    user := &User{Name: "John", Email: "john@example.com"}

    err := repo.SaveUser(user)
    assert.NoError(t, err)
    assert.NotZero(t, user.ID)
}
```

## Test Fixtures

```go
type testFixtures struct {
    db    *sql.DB
    users []*User
}

func setupFixtures(t *testing.T) *testFixtures {
    db := setupTestDB(t)

    users := []*User{
        {Name: "John", Email: "john@example.com"},
        {Name: "Jane", Email: "jane@example.com"},
    }

    repo := NewUserRepository(db)
    for _, user := range users {
        err := repo.SaveUser(user)
        require.NoError(t, err)
    }

    return &testFixtures{
        db:    db,
        users: users,
    }
}

func TestWithFixtures(t *testing.T) {
    fixtures := setupFixtures(t)
    defer fixtures.db.Close()

    repo := NewUserRepository(fixtures.db)
    user, err := repo.GetUser(fixtures.users[0].ID)

    assert.NoError(t, err)
    assert.Equal(t, "John", user.Name)
}
```

## Testing Repository Layer

```go
func TestUserRepository_GetUser(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    repo := NewUserRepository(db)

    tests := []struct {
        name      string
        setup     func() int
        userID    int
        expectErr bool
    }{
        {
            name: "existing user",
            setup: func() int {
                user := &User{Name: "John", Email: "john@example.com"}
                repo.SaveUser(user)
                return user.ID
            },
            expectErr: false,
        },
        {
            name: "non-existing user",
            setup: func() int {
                return 999999
            },
            userID:    999999,
            expectErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tx, _ := db.Begin()
            defer tx.Rollback()

            var userID int
            if tt.setup != nil {
                userID = tt.setup()
            } else {
                userID = tt.userID
            }

            user, err := repo.GetUser(userID)

            if tt.expectErr {
                assert.Error(t, err)
                assert.Nil(t, user)
            } else {
                assert.NoError(t, err)
                assert.NotNil(t, user)
            }
        })
    }
}
```

## Helper Functions

```go
func setupTestDB(t *testing.T) *sql.DB {
    t.Helper()

    // Setup testcontainer or in-memory DB
    db := createTestDatabase(t)

    // Run migrations
    err := runMigrations(db)
    require.NoError(t, err)

    // Cleanup
    t.Cleanup(func() {
        db.Close()
    })

    return db
}

func runMigrations(db *sql.DB) error {
    // Run migration files
    migrations := []string{
        `CREATE TABLE users (
            id SERIAL PRIMARY KEY,
            name TEXT NOT NULL,
            email TEXT UNIQUE NOT NULL,
            created_at TIMESTAMP DEFAULT NOW()
        )`,
    }

    for _, migration := range migrations {
        _, err := db.Exec(migration)
        if err != nil {
            return err
        }
    }

    return nil
}

func truncateTables(t *testing.T, db *sql.DB) {
    t.Helper()

    tables := []string{"users", "orders", "products"}
    for _, table := range tables {
        _, err := db.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
        require.NoError(t, err)
    }
}
```

## Testing with pgx

```go
import (
    "github.com/jackc/pgx/v5/pgxpool"
)

func TestUserRepositoryPgx(t *testing.T) {
    ctx := context.Background()
    container := setupPostgresContainer(t)
    defer container.Terminate(ctx)

    dsn := getContainerDSN(container)
    pool, err := pgxpool.New(ctx, dsn)
    require.NoError(t, err)
    defer pool.Close()

    repo := NewUserRepository(pool)

    t.Run("create user", func(t *testing.T) {
        user := &User{Name: "John", Email: "john@example.com"}
        err := repo.SaveUser(ctx, user)

        assert.NoError(t, err)
        assert.NotZero(t, user.ID)
    })

    t.Run("get user", func(t *testing.T) {
        user, err := repo.GetUser(ctx, 1)

        assert.NoError(t, err)
        assert.Equal(t, "John", user.Name)
    })
}
```

## Testing Concurrent Access

```go
func TestConcurrentAccess(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    repo := NewUserRepository(db)
    user := &User{Name: "John", Email: "john@example.com"}
    repo.SaveUser(user)

    var wg sync.WaitGroup
    errors := make(chan error, 10)

    // Concurrent reads
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            _, err := repo.GetUser(user.ID)
            if err != nil {
                errors <- err
            }
        }()
    }

    wg.Wait()
    close(errors)

    // Check for errors
    for err := range errors {
        t.Errorf("concurrent access error: %v", err)
    }
}
```

## Best Practices

1. **Use testcontainers** - Test against real database
2. **Isolate tests with transactions** - Rollback after each test
3. **Use fixtures for complex scenarios** - Setup common test data
4. **Test both success and failure cases** - Including constraints and errors
5. **Clean up resources** - Use t.Cleanup() or defer
6. **Test concurrent access** - Verify thread safety
7. **Run migrations in tests** - Ensure schema matches production
8. **Use connection pooling** - Match production configuration
9. **Test database constraints** - Verify unique, foreign key, etc.
10. **Mock external databases in unit tests** - Use real DB for integration tests

## Running Database Tests

```bash
# Run all tests including integration tests
go test ./...

# Run only unit tests (skip integration)
go test -short ./...

# Run with race detector
go test -race ./...

# Run specific test
go test -run TestUserRepository
```

## Marking Integration Tests

```go
func TestIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // Integration test code
}
```

Run unit tests only:
```bash
go test -short ./...
```
