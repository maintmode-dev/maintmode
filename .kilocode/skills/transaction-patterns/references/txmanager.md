# TxManager Implementation

Complete guide to the TxManager pattern used in MaintMode.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Component Breakdown](#component-breakdown)
3. [Transaction Lifecycle](#transaction-lifecycle)
4. [Implementation Details](#implementation-details)
5. [Advanced Scenarios](#advanced-scenarios)

## Architecture Overview

The TxManager pattern consists of three components working together:

```
┌─────────────┐
│  TxManager  │  Manages lifecycle
└──────┬──────┘
       │
       ├─────> Context (txKey)  Propagates transaction
       │
       └─────> Executor         Routes queries
```

### Component Responsibilities

**TxManager** (`tx_manager.go`):
- Begins transactions with proper isolation level
- Wraps user code in transaction boundary
- Handles commit/rollback
- Recovers from panics

**Context Keys** (`txctx.go`):
- Type-safe storage of transaction in context
- Extraction of transaction from context
- Manual transaction injection (testing)

**Executor** (`executor.go`):
- Returns transaction if present in context
- Falls back to database connection otherwise
- Single interface for stores

## Component Breakdown

### TxManager

```go
type TxManager struct {
    db *sqlx.DB
}

func NewTxManager(db *sqlx.DB) *TxManager {
    return &TxManager{db: db}
}

func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
    // 1. Begin transaction
    tx, err := m.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault})
    if err != nil {
        return err
    }

    // 2. Store in context
    ctx = context.WithValue(ctx, txKey{}, tx)

    // 3. Panic recovery
    defer func() {
        if recErr := recover(); recErr != nil {
            xlog.Error(ctx, "panic recovery when execute the transaction",
                zap.Any("panic", recErr))
        }
    }()

    // 4. Execute user function
    if err := fn(ctx); err != nil {
        xlog.Error(ctx, "raise error when execute the transaction", zap.Error(err))
        _ = rollback(ctx, tx)
        return err
    }

    // 5. Commit
    return commit(ctx, tx)
}
```

**Key Design Decisions:**

1. **Default Isolation Level**: Uses `sql.LevelDefault` (Read Committed in PostgreSQL)
   - Good balance between consistency and performance
   - Prevents dirty reads
   - Allows phantom reads (acceptable for most use cases)

2. **Panic Recovery**: Catches panics to prevent transaction leaks
   - Logs panic for debugging
   - Transaction is rolled back (defer cleanup in driver)
   - Re-panic could be added if needed

3. **Error Handling**: Clear separation of concerns
   - Logs all errors with context
   - Rollback errors are logged but not returned (original error preserved)
   - Commit errors are returned to caller

### Transaction Context

```go
type txKey struct{}

func WithTx(ctx context.Context, tx *sqlx.Tx) context.Context {
    return context.WithValue(ctx, txKey{}, tx)
}

func TxFromContext(ctx context.Context) (*sqlx.Tx, bool) {
    tx, ok := ctx.Value(txKey{}).(*sqlx.Tx)
    return tx, ok
}
```

**Key Design Decisions:**

1. **Empty Struct Key**: `txKey struct{}` ensures key uniqueness
   - No memory overhead
   - Cannot be created outside package (unexported)
   - Type-safe lookup

2. **Explicit Return**: Returns `(tx, ok)` tuple
   - Allows checking if transaction exists
   - Prevents nil pointer panics
   - Clear intent

### Executor

```go
type DB struct {
    db *sqlx.DB
}

func NewDB(db *sqlx.DB) *DB {
    return &DB{db: db}
}

func (t *DB) Executor(ctx context.Context) sqlx.ExtContext {
    if tx, ok := TxFromContext(ctx); ok {
        return tx
    }
    return t.db
}
```

**Key Design Decisions:**

1. **Returns Interface**: `sqlx.ExtContext` is implemented by both `*sqlx.DB` and `*sqlx.Tx`
   - Stores don't need to know which they're using
   - Same query API regardless
   - Simplifies testing

2. **Context Inspection**: Checks context on every call
   - No state to manage
   - Thread-safe
   - Works with any context flow

## Transaction Lifecycle

### Successful Transaction

```
┌──────────────────────────────────────────────────────────┐
│ 1. WithinTx called                                       │
│    - Begin transaction with isolation level              │
│    - Store tx in context                                 │
└─────────────────────┬────────────────────────────────────┘
                      │
                      ▼
┌──────────────────────────────────────────────────────────┐
│ 2. Execute fn(ctx)                                       │
│    - User code runs                                      │
│    - All queries use transaction via Executor            │
│    - Returns nil (success)                               │
└─────────────────────┬────────────────────────────────────┘
                      │
                      ▼
┌──────────────────────────────────────────────────────────┐
│ 3. Commit transaction                                    │
│    - tx.Commit() called                                  │
│    - Changes become visible                              │
│    - Locks released                                      │
└──────────────────────────────────────────────────────────┘
```

### Failed Transaction

```
┌──────────────────────────────────────────────────────────┐
│ 1. WithinTx called                                       │
│    - Begin transaction                                   │
│    - Store tx in context                                 │
└─────────────────────┬────────────────────────────────────┘
                      │
                      ▼
┌──────────────────────────────────────────────────────────┐
│ 2. Execute fn(ctx)                                       │
│    - Query returns error                                 │
│    - Error propagated to WithinTx                        │
└─────────────────────┬────────────────────────────────────┘
                      │
                      ▼
┌──────────────────────────────────────────────────────────┐
│ 3. Rollback transaction                                  │
│    - tx.Rollback() called                                │
│    - All changes discarded                               │
│    - Locks released                                      │
└──────────────────────┬────────────────────────────────────┘
                      │
                      ▼
┌──────────────────────────────────────────────────────────┐
│ 4. Return error to caller                                │
└──────────────────────────────────────────────────────────┘
```

### Panic Recovery

```
┌──────────────────────────────────────────────────────────┐
│ 1. WithinTx called                                       │
│    - Begin transaction                                   │
│    - Set up defer for panic recovery                     │
└─────────────────────┬────────────────────────────────────┘
                      │
                      ▼
┌──────────────────────────────────────────────────────────┐
│ 2. Execute fn(ctx)                                       │
│    - Code panics                                         │
│    - Defer triggered                                     │
└─────────────────────┬────────────────────────────────────┘
                      │
                      ▼
┌──────────────────────────────────────────────────────────┐
│ 3. Panic recovery                                        │
│    - recover() captures panic                            │
│    - Panic logged with context                           │
│    - Transaction rolled back (driver cleanup)            │
└──────────────────────────────────────────────────────────┘
```

## Implementation Details

### Custom Isolation Levels

To use different isolation levels for specific operations:

```go
func (m *TxManager) WithinTxSerializable(ctx context.Context, fn func(ctx context.Context) error) error {
    tx, err := m.db.BeginTxx(ctx, &sql.TxOptions{
        Isolation: sql.LevelSerializable,
    })
    if err != nil {
        return err
    }

    ctx = context.WithValue(ctx, txKey{}, tx)

    defer func() {
        if recErr := recover(); recErr != nil {
            xlog.Error(ctx, "panic in serializable transaction", zap.Any("panic", recErr))
        }
    }()

    if err := fn(ctx); err != nil {
        _ = rollback(ctx, tx)
        return err
    }

    return commit(ctx, tx)
}
```

**When to use:**
- Critical financial operations
- Operations requiring strict consistency
- Preventing phantom reads
- Sequential processing requirements

**Trade-offs:**
- Increased serialization failure rate
- Performance impact from additional locking
- May require retry logic

### Read-Only Transactions

For queries that only read data:

```go
func (m *TxManager) WithinTxReadOnly(ctx context.Context, fn func(ctx context.Context) error) error {
    tx, err := m.db.BeginTxx(ctx, &sql.TxOptions{
        Isolation: sql.LevelDefault,
        ReadOnly:  true,
    })
    if err != nil {
        return err
    }

    ctx = context.WithValue(ctx, txKey{}, tx)

    defer func() {
        if recErr := recover(); recErr != nil {
            xlog.Error(ctx, "panic in read-only transaction", zap.Any("panic", recErr))
        }
    }()

    if err := fn(ctx); err != nil {
        _ = rollback(ctx, tx)
        return err
    }

    return commit(ctx, tx)
}
```

**Benefits:**
- PostgreSQL can optimize read-only transactions
- Prevents accidental writes
- Useful for complex reports requiring consistency
- Can potentially use read replicas

**When to use:**
- Multi-step read operations requiring consistent snapshot
- Complex reports joining many tables
- Operations that should never modify data

### Timeout Support

Add transaction timeout to prevent long-running transactions:

```go
func (m *TxManager) WithinTxTimeout(ctx context.Context, timeout time.Duration, fn func(ctx context.Context) error) error {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    tx, err := m.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault})
    if err != nil {
        return err
    }

    ctx = context.WithValue(ctx, txKey{}, tx)

    defer func() {
        if recErr := recover(); recErr != nil {
            xlog.Error(ctx, "panic in transaction", zap.Any("panic", recErr))
        }
    }()

    if err := fn(ctx); err != nil {
        _ = rollback(ctx, tx)
        return err
    }

    return commit(ctx, tx)
}
```

**Usage:**

```go
// Timeout after 5 seconds
err := s.txManager.WithinTxTimeout(ctx, 5*time.Second, func(ctx context.Context) error {
    // Transaction operations
    return nil
})
```

## Advanced Scenarios

### Handling Deadlocks

PostgreSQL automatically detects deadlocks and aborts one transaction:

```go
func (s *Service) UpdateWithRetry(ctx context.Context, id uuid.UUID) error {
    const maxRetries = 3
    var err error

    for attempt := 0; attempt < maxRetries; attempt++ {
        err = s.txManager.WithinTx(ctx, func(ctx context.Context) error {
            // Operations that might deadlock
            return s.store.Update(ctx, id)
        })

        if err == nil {
            return nil
        }

        // Check if deadlock (PostgreSQL error code 40P01)
        var pgErr *pgconn.PgError
        if errors.As(err, &pgErr) && pgErr.Code == "40P01" {
            // Deadlock detected, retry with backoff
            time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
            continue
        }

        // Other error, don't retry
        return err
    }

    return fmt.Errorf("failed after %d retries: %w", maxRetries, err)
}
```

### Transaction Metrics

Add metrics to monitor transaction performance:

```go
func (m *TxManager) WithinTxMetrics(ctx context.Context, fn func(ctx context.Context) error) error {
    start := time.Now()

    tx, err := m.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault})
    if err != nil {
        return err
    }

    ctx = context.WithValue(ctx, txKey{}, tx)

    defer func() {
        duration := time.Since(start)

        if recErr := recover(); recErr != nil {
            metrics.TransactionPanics.Inc()
            xlog.Error(ctx, "panic in transaction",
                zap.Any("panic", recErr),
                zap.Duration("duration", duration))
        }
    }()

    if err := fn(ctx); err != nil {
        metrics.TransactionRollbacks.Inc()
        metrics.TransactionDuration.Observe(time.Since(start).Seconds())
        _ = rollback(ctx, tx)
        return err
    }

    if err := commit(ctx, tx); err != nil {
        metrics.TransactionCommitFailures.Inc()
        return err
    }

    metrics.TransactionCommits.Inc()
    metrics.TransactionDuration.Observe(time.Since(start).Seconds())
    return nil
}
```

### Savepoints (Partial Rollback)

PostgreSQL supports savepoints for partial rollback within transactions:

```go
func (m *TxManager) executeSavepoint(ctx context.Context, tx *sqlx.Tx, name string) error {
    _, err := tx.ExecContext(ctx, fmt.Sprintf("SAVEPOINT %s", name))
    return err
}

func (m *TxManager) rollbackToSavepoint(ctx context.Context, tx *sqlx.Tx, name string) error {
    _, err := tx.ExecContext(ctx, fmt.Sprintf("ROLLBACK TO SAVEPOINT %s", name))
    return err
}

// Usage in service
err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
    // Main operation
    if err := s.store.Create(ctx, mainEntity); err != nil {
        return err
    }

    // Get transaction for savepoint
    tx, ok := dbtx.TxFromContext(ctx)
    if !ok {
        return errors.New("no transaction in context")
    }

    // Create savepoint before risky operation
    if err := s.executeSavepoint(ctx, tx, "before_optional"); err != nil {
        return err
    }

    // Optional operation that might fail
    if err := s.store.CreateOptional(ctx, optionalEntity); err != nil {
        // Rollback just this operation
        _ = s.rollbackToSavepoint(ctx, tx, "before_optional")
        xlog.Warn(ctx, "optional operation failed, continuing", zap.Error(err))
    }

    // Continue with main transaction
    return nil
})
```

**Use cases:**
- Optional operations within transactions
- Batch operations with partial success
- Retry logic for specific operations

**Caution:**
- Adds complexity
- Can mask errors
- Consider if separate transactions might be better
