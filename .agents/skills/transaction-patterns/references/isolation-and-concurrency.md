# Isolation Levels & Concurrency

Understanding PostgreSQL transaction isolation and handling concurrent access patterns.

## Table of Contents

1. [Isolation Levels](#isolation-levels)
2. [Concurrency Phenomena](#concurrency-phenomena)
3. [Locking Strategies](#locking-strategies)
4. [Conflict Detection](#conflict-detection)
5. [Retry Strategies](#retry-strategies)

## Isolation Levels

PostgreSQL supports four standard isolation levels, but implements only three due to its MVCC architecture.

### Read Committed (Default)

**What it prevents**: Dirty reads
**What it allows**: Non-repeatable reads, phantom reads
**Performance**: Best
**Use case**: Most operations

```go
tx, err := db.BeginTxx(ctx, &sql.TxOptions{
    Isolation: sql.LevelReadCommitted, // or sql.LevelDefault
})
```

**Behavior:**

```sql
-- Session 1
BEGIN;
SELECT balance FROM accounts WHERE id = 1; -- Returns 100

-- Session 2
BEGIN;
UPDATE accounts SET balance = 200 WHERE id = 1;
COMMIT;

-- Session 1 (same transaction)
SELECT balance FROM accounts WHERE id = 1; -- Returns 200 (!)
-- Non-repeatable read: same query, different result
```

**When to use:**
- Default choice for most operations
- Good balance of consistency and performance
- Prevents seeing uncommitted data
- Acceptable for operations that don't require strict consistency

**Real example in MaintMode:**

```go
func (s *Service) GetMaintenanceList(ctx context.Context, filter *Filter) ([]*Maintenance, error) {
    // Read Committed is fine - we're just displaying data
    // If another transaction commits during our query, that's acceptable
    return s.store.List(ctx, filter)
}
```

### Repeatable Read

**What it prevents**: Dirty reads, non-repeatable reads
**What it allows**: Phantom reads (but prevents them with MVCC in practice)
**Performance**: Good
**Use case**: Operations requiring consistent snapshot

```go
tx, err := db.BeginTxx(ctx, &sql.TxOptions{
    Isolation: sql.LevelRepeatableRead,
})
```

**Behavior:**

```sql
-- Session 1
BEGIN ISOLATION LEVEL REPEATABLE READ;
SELECT balance FROM accounts WHERE id = 1; -- Returns 100

-- Session 2
BEGIN;
UPDATE accounts SET balance = 200 WHERE id = 1;
COMMIT;

-- Session 1 (same transaction)
SELECT balance FROM accounts WHERE id = 1; -- Still returns 100
-- Repeatable read: same query always returns same result
```

**When to use:**
- Long-running read operations requiring consistency
- Reports that join multiple tables
- Operations that read, compute, then write
- Batch processing with consistent view

**PostgreSQL special behavior:**

Unlike standard SQL, PostgreSQL's implementation prevents phantom reads too (via MVCC), making this isolation level stronger than the SQL standard requires.

**Implementation:**

```go
func (m *TxManager) WithinTxRepeatableRead(ctx context.Context, fn func(ctx context.Context) error) error {
    tx, err := m.db.BeginTxx(ctx, &sql.TxOptions{
        Isolation: sql.LevelRepeatableRead,
    })
    if err != nil {
        return err
    }

    ctx = context.WithValue(ctx, txKey{}, tx)

    defer func() {
        if recErr := recover(); recErr != nil {
            xlog.Error(ctx, "panic in repeatable read transaction",
                zap.Any("panic", recErr))
        }
    }()

    if err := fn(ctx); err != nil {
        _ = rollback(ctx, tx)
        return err
    }

    return commit(ctx, tx)
}
```

**Real example:**

```go
func (s *Service) GenerateReport(ctx context.Context, period Period) (*Report, error) {
    var report *Report

    err := s.txManager.WithinTxRepeatableRead(ctx, func(ctx context.Context) error {
        // All queries see consistent snapshot
        maintenances, err := s.maintStore.FindByPeriod(ctx, period)
        if err != nil {
            return err
        }

        resources, err := s.resourceStore.GetAll(ctx)
        if err != nil {
            return err
        }

        assignments, err := s.assignmentStore.FindByPeriod(ctx, period)
        if err != nil {
            return err
        }

        report = buildReport(maintenances, resources, assignments)
        return nil
    })

    return report, err
}
```

### Serializable

**What it prevents**: All concurrency phenomena
**What it allows**: Nothing - full isolation
**Performance**: Lowest (serialization failures)
**Use case**: Critical operations requiring strict consistency

```go
tx, err := db.BeginTxx(ctx, &sql.TxOptions{
    Isolation: sql.LevelSerializable,
})
```

**Behavior:**

PostgreSQL uses Serializable Snapshot Isolation (SSI), which detects conflicts and aborts one of the conflicting transactions.

```sql
-- Session 1
BEGIN ISOLATION LEVEL SERIALIZABLE;
SELECT SUM(balance) FROM accounts; -- Returns 1000

-- Session 2
BEGIN ISOLATION LEVEL SERIALIZABLE;
SELECT SUM(balance) FROM accounts; -- Returns 1000
INSERT INTO accounts (id, balance) VALUES (999, 100);
COMMIT; -- Success

-- Session 1
INSERT INTO accounts (id, balance) VALUES (998, 100);
COMMIT; -- ERROR: could not serialize access
-- PostgreSQL detected that concurrent execution would violate serializability
```

**When to use:**
- Financial transactions
- Inventory management with strict consistency
- Operations where race conditions would cause data corruption
- Sequential processing requirements

**Must implement retry logic:**

```go
func (s *Service) TransferFunds(ctx context.Context, from, to uuid.UUID, amount decimal.Decimal) error {
    return s.withRetryOnSerializationFailure(ctx, func(ctx context.Context) error {
        return s.txManager.WithinTxSerializable(ctx, func(ctx context.Context) error {
            // Deduct from source
            fromAccount, err := s.accountStore.GetForUpdate(ctx, from)
            if err != nil {
                return err
            }

            if fromAccount.Balance.LessThan(amount) {
                return ErrInsufficientFunds
            }

            fromAccount.Balance = fromAccount.Balance.Sub(amount)
            if err := s.accountStore.Update(ctx, fromAccount); err != nil {
                return err
            }

            // Add to destination
            toAccount, err := s.accountStore.GetForUpdate(ctx, to)
            if err != nil {
                return err
            }

            toAccount.Balance = toAccount.Balance.Add(amount)
            if err := s.accountStore.Update(ctx, toAccount); err != nil {
                return err
            }

            return nil
        })
    })
}

func (s *Service) withRetryOnSerializationFailure(ctx context.Context, fn func(context.Context) error) error {
    const maxRetries = 5

    for attempt := 0; attempt < maxRetries; attempt++ {
        err := fn(ctx)
        if err == nil {
            return nil
        }

        // Check for serialization failure (40001)
        var pgErr *pgconn.PgError
        if errors.As(err, &pgErr) && pgErr.Code == "40001" {
            // Exponential backoff
            time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * 10 * time.Millisecond)
            continue
        }

        return err
    }

    return fmt.Errorf("transaction failed after %d retries", maxRetries)
}
```

## Concurrency Phenomena

### Dirty Read

Reading uncommitted data from another transaction.

**Example:**

```sql
-- Session 1
BEGIN;
UPDATE accounts SET balance = 1000 WHERE id = 1;
-- Not committed yet

-- Session 2
BEGIN;
SELECT balance FROM accounts WHERE id = 1; -- Sees 1000 (DIRTY READ)

-- Session 1
ROLLBACK; -- Oops, that update was wrong
```

**Prevention**: All PostgreSQL isolation levels prevent dirty reads.

### Non-Repeatable Read

Reading the same row twice returns different values.

**Example:**

```sql
-- Session 1
BEGIN;
SELECT balance FROM accounts WHERE id = 1; -- Returns 100

-- Session 2
UPDATE accounts SET balance = 200 WHERE id = 1;
COMMIT;

-- Session 1
SELECT balance FROM accounts WHERE id = 1; -- Returns 200 (NON-REPEATABLE)
```

**Prevention**: Repeatable Read and Serializable isolation levels.

**When it matters:**

```go
func (s *Service) ProcessMaintenance(ctx context.Context, id uuid.UUID) error {
    // Read Committed - might see different status between reads
    maint1, _ := s.store.Get(ctx, id)
    if maint1.Status != StatusDraft {
        return ErrInvalidStatus
    }

    // Another transaction might change status here!

    // This read might see different status
    maint2, _ := s.store.Get(ctx, id)
    // maint2.Status could be different!
}
```

**Solution**: Use transaction with Repeatable Read, or lock the row:

```go
func (s *Service) ProcessMaintenance(ctx context.Context, id uuid.UUID) error {
    return s.txManager.WithinTx(ctx, func(ctx context.Context) error {
        // Lock row - prevents concurrent modifications
        maint, err := s.store.GetForUpdate(ctx, id)
        if err != nil {
            return err
        }

        if maint.Status != StatusDraft {
            return ErrInvalidStatus
        }

        // Safe - no one can change status while we hold lock
        maint.Status = StatusScheduled
        return s.store.Update(ctx, maint)
    })
}
```

### Phantom Read

Query returns different set of rows when executed twice.

**Example:**

```sql
-- Session 1
BEGIN;
SELECT COUNT(*) FROM maintenances WHERE status = 'draft'; -- Returns 5

-- Session 2
INSERT INTO maintenances (id, status) VALUES (uuid_generate_v4(), 'draft');
COMMIT;

-- Session 1
SELECT COUNT(*) FROM maintenances WHERE status = 'draft'; -- Returns 6 (PHANTOM)
```

**Prevention**: Repeatable Read (in PostgreSQL due to MVCC) and Serializable.

## Locking Strategies

### Optimistic Locking

Assume conflicts are rare; detect and handle them when they occur.

**Implementation with version column:**

```go
type Maintenance struct {
    ID        uuid.UUID
    Title     string
    Status    Status
    Version   int  // Optimistic lock version
    UpdatedAt time.Time
}

func (s *Store) Update(ctx context.Context, maint *Maintenance) error {
    stmt := table.Maintenances.
        UPDATE(
            table.Maintenances.Title,
            table.Maintenances.Status,
            table.Maintenances.Version,
            table.Maintenances.UpdatedAt,
        ).
        SET(
            postgres.String(maint.Title),
            postgres.String(string(maint.Status)),
            postgres.Int(maint.Version + 1),
            postgres.TimestampT(time.Now()),
        ).
        WHERE(
            table.Maintenances.ID.EQ(postgres.UUID(maint.ID)).
            AND(table.Maintenances.Version.EQ(postgres.Int(maint.Version))),
        )

    result, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
    if err != nil {
        return err
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return err
    }

    if rows == 0 {
        return ErrOptimisticLockFailed // Someone else modified it
    }

    maint.Version++
    return nil
}
```

**When to use:**
- Low contention scenarios
- Read-heavy workloads
- Operations that can be retried
- Updates from user input (could take time)

**Pros:**
- No locks held during user interaction
- Better concurrency
- No deadlock risk

**Cons:**
- Retry logic required
- User might lose work
- More complex error handling

### Pessimistic Locking

Lock rows immediately to prevent concurrent access.

**Implementation with SELECT FOR UPDATE:**

```go
func (s *Store) GetForUpdate(ctx context.Context, id uuid.UUID) (*entity.Maintenance, error) {
    stmt := table.Maintenances.
        SELECT(table.Maintenances.AllColumns).
        WHERE(table.Maintenances.ID.EQ(postgres.UUID(id))).
        FOR(postgres.UPDATE())  // Locks the row

    maint := new(model.Maintenances)
    err := stmt.QueryContext(ctx, s.db.Executor(ctx), maint)
    if err != nil {
        if errors.Is(err, qrm.ErrNoRows) {
            return nil, apperr.ErrMaintNotFound
        }
        return nil, err
    }

    return fromDBMaintenance(maint), nil
}
```

**Lock modes:**

```go
// Exclusive lock - blocks all other access
.FOR(postgres.UPDATE())

// Share lock - allows concurrent reads but blocks writes
.FOR(postgres.SHARE())

// Exclusive, skip locked rows
.FOR(postgres.UPDATE().SKIP_LOCKED())

// Exclusive, fail immediately if locked
.FOR(postgres.UPDATE().NOWAIT())
```

**When to use:**
- High contention scenarios
- Critical updates that must succeed
- Sequential processing requirements
- Financial transactions

**Pros:**
- Guaranteed no conflicts
- Simpler logic (no retry)
- Immediate feedback if row is locked

**Cons:**
- Reduces concurrency
- Risk of deadlocks
- Can cause lock waits

### Lock Granularity

**Row-level locks** (SELECT FOR UPDATE):
```go
// Only locks the specific maintenance record
maint, err := s.store.GetForUpdate(ctx, id)
```

**Table-level locks** (rare, use with caution):
```sql
LOCK TABLE maintenances IN EXCLUSIVE MODE;
```

**Advisory locks** (application-level coordination):
```sql
SELECT pg_advisory_xact_lock(123); -- Lock number 123
```

## Conflict Detection

### Detecting Serialization Failures

```go
func isSerializationFailure(err error) bool {
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        // 40001: serialization_failure
        // 40P01: deadlock_detected
        return pgErr.Code == "40001" || pgErr.Code == "40P01"
    }
    return false
}
```

### Detecting Lock Timeout

```go
func isLockTimeout(err error) bool {
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        // 55P03: lock_not_available
        return pgErr.Code == "55P03"
    }
    return false
}
```

### Comprehensive Error Handling

```go
func (s *Service) UpdateWithConflictHandling(ctx context.Context, id uuid.UUID, update *Update) error {
    err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
        maint, err := s.store.GetForUpdate(ctx, id)
        if err != nil {
            return err
        }

        maint.ApplyUpdate(update)
        return s.store.Update(ctx, maint)
    })

    if err == nil {
        return nil
    }

    // Handle specific error types
    switch {
    case isSerializationFailure(err):
        return apperr.ErrConcurrentModification
    case isLockTimeout(err):
        return apperr.ErrResourceLocked
    case errors.Is(err, ErrOptimisticLockFailed):
        return apperr.ErrConcurrentModification
    default:
        return err
    }
}
```

## Retry Strategies

### Exponential Backoff

```go
func (s *Service) UpdateWithRetry(ctx context.Context, id uuid.UUID, update *Update) error {
    const (
        maxRetries     = 5
        initialBackoff = 10 * time.Millisecond
        maxBackoff     = 1 * time.Second
    )

    var lastErr error
    for attempt := 0; attempt < maxRetries; attempt++ {
        err := s.Update(ctx, id, update)
        if err == nil {
            return nil
        }

        lastErr = err

        // Only retry on certain errors
        if !isRetryable(err) {
            return err
        }

        // Calculate backoff with jitter
        backoff := time.Duration(math.Pow(2, float64(attempt))) * initialBackoff
        if backoff > maxBackoff {
            backoff = maxBackoff
        }

        // Add jitter (random 0-50%)
        jitter := time.Duration(rand.Float64() * float64(backoff) * 0.5)
        time.Sleep(backoff + jitter)

        xlog.Warn(ctx, "retrying after conflict",
            zap.Int("attempt", attempt+1),
            zap.Error(err))
    }

    return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

func isRetryable(err error) bool {
    return isSerializationFailure(err) ||
           errors.Is(err, ErrOptimisticLockFailed) ||
           isDeadlock(err)
}
```

### Retry with Context Timeout

```go
func (s *Service) UpdateWithRetryAndTimeout(ctx context.Context, id uuid.UUID, update *Update) error {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    for {
        select {
        case <-ctx.Done():
            return fmt.Errorf("update timed out: %w", ctx.Err())
        default:
        }

        err := s.Update(ctx, id, update)
        if err == nil {
            return nil
        }

        if !isRetryable(err) {
            return err
        }

        // Short sleep before retry
        select {
        case <-ctx.Done():
            return fmt.Errorf("update timed out: %w", ctx.Err())
        case <-time.After(50 * time.Millisecond):
        }
    }
}
```

## Best Practices

1. **Start with Read Committed**: Use default isolation for most operations
2. **Use Repeatable Read for Reports**: Consistent snapshots for multi-query operations
3. **Reserve Serializable**: Only for critical operations with retry logic
4. **Prefer Optimistic Locking**: Better concurrency in low-contention scenarios
5. **Use FOR UPDATE Judiciously**: Only when necessary, keep lock duration short
6. **Implement Retry Logic**: Always handle serialization failures
7. **Set Lock Timeouts**: Prevent indefinite waits
8. **Monitor Deadlocks**: Track and analyze deadlock patterns
9. **Keep Transactions Short**: Minimize lock duration
10. **Test Concurrency**: Use race condition testing tools
