# Error Handling & Rollback

Comprehensive error handling strategies for transactional operations.

## Table of Contents

1. [Error Categories](#error-categories)
2. [Rollback Strategies](#rollback-strategies)
3. [Error Propagation](#error-propagation)
4. [Partial Success Handling](#partial-success-handling)
5. [Recovery Patterns](#recovery-patterns)

## Error Categories

### Database Errors

**Connection Errors**:
```go
func isDatabaseConnectionError(err error) bool {
    if err == nil {
        return false
    }

    // Driver-level connection errors
    if errors.Is(err, driver.ErrBadConn) {
        return true
    }

    // PostgreSQL connection errors
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        // Class 08: Connection Exception
        return strings.HasPrefix(pgErr.Code, "08")
    }

    return false
}
```

**Constraint Violations**:
```go
func isConstraintViolation(err error) bool {
    var pgErr *pgconn.PgError
    if !errors.As(err, &pgErr) {
        return false
    }

    switch pgErr.Code {
    case "23000": // integrity_constraint_violation
        return true
    case "23001": // restrict_violation
        return true
    case "23502": // not_null_violation
        return true
    case "23503": // foreign_key_violation
        return true
    case "23505": // unique_violation
        return true
    case "23514": // check_violation
        return true
    case "23P01": // exclusion_violation
        return true
    default:
        return false
    }
}

func parseConstraintError(err error) *ConstraintError {
    var pgErr *pgconn.PgError
    if !errors.As(err, &pgErr) {
        return nil
    }

    return &ConstraintError{
        Code:       pgErr.Code,
        Constraint: pgErr.ConstraintName,
        Table:      pgErr.TableName,
        Column:     pgErr.ColumnName,
        Detail:     pgErr.Detail,
    }
}
```

**Transaction Errors**:
```go
func isTransactionError(err error) bool {
    var pgErr *pgconn.PgError
    if !errors.As(err, &pgErr) {
        return false
    }

    switch pgErr.Code {
    case "40001": // serialization_failure
        return true
    case "40P01": // deadlock_detected
        return true
    case "55P03": // lock_not_available
        return true
    case "25001": // active_sql_transaction (trying to begin within transaction)
        return true
    case "25P02": // in_failed_sql_transaction
        return true
    default:
        return false
    }
}
```

### Application Errors

**Business Logic Errors**:
```go
// Domain-specific errors
var (
    ErrInvalidStatus              = errors.New("invalid status")
    ErrInsufficientPermissions    = errors.New("insufficient permissions")
    ErrResourceNotAvailable       = errors.New("resource not available")
    ErrOverlappingMaintenance     = errors.New("overlapping maintenance window")
)

// Should trigger rollback but not retry
func isBusinessError(err error) bool {
    return errors.Is(err, ErrInvalidStatus) ||
           errors.Is(err, ErrInsufficientPermissions) ||
           errors.Is(err, ErrResourceNotAvailable) ||
           errors.Is(err, ErrOverlappingMaintenance)
}
```

**Validation Errors**:
```go
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed on field %s: %s", e.Field, e.Message)
}

func isValidationError(err error) bool {
    var valErr *ValidationError
    return errors.As(err, &valErr)
}
```

## Rollback Strategies

### Automatic Rollback

The TxManager automatically rolls back on any error:

```go
func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
    tx, err := m.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault})
    if err != nil {
        return err
    }

    ctx = context.WithValue(ctx, txKey{}, tx)

    defer func() {
        if recErr := recover(); recErr != nil {
            xlog.Error(ctx, "panic recovery", zap.Any("panic", recErr))
        }
    }()

    // Execute function
    if err := fn(ctx); err != nil {
        xlog.Error(ctx, "transaction error, rolling back", zap.Error(err))
        _ = rollback(ctx, tx)  // Rollback on error
        return err
    }

    return commit(ctx, tx)
}
```

**Key principle**: Return any error directly to trigger rollback. Don't try to handle rollback manually.

### Decision Tree for Rollback

```
Error occurred
    │
    ├─ Connection error?
    │   └─ Yes → Rollback + Retry with new connection
    │
    ├─ Serialization failure?
    │   └─ Yes → Rollback + Retry transaction
    │
    ├─ Deadlock?
    │   └─ Yes → Rollback + Retry with backoff
    │
    ├─ Constraint violation?
    │   └─ Yes → Rollback + Return user error
    │
    ├─ Validation error?
    │   └─ Yes → Don't start transaction
    │
    └─ Business logic error?
        └─ Yes → Rollback + Return domain error
```

### Rollback with Logging

```go
func rollback(ctx context.Context, tx *sqlx.Tx) error {
    if err := tx.Rollback(); err != nil {
        // Special case: already rolled back or committed
        if errors.Is(err, sql.ErrTxDone) {
            xlog.Warn(ctx, "transaction already finished")
            return nil
        }

        xlog.Error(ctx, "failed to rollback transaction",
            zap.Error(err),
            zap.Stack("stack"))
        return err
    }

    xlog.Info(ctx, "transaction rolled back successfully")
    return nil
}
```

### Rollback with Cleanup

For operations with side effects:

```go
func (s *Service) CreateWithFile(ctx context.Context, cmd *CreateCmd) error {
    var filePath string

    err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
        // Create database record
        entity, err := s.store.Create(ctx, cmd.Entity)
        if err != nil {
            return err
        }

        // Upload file
        filePath, err = s.fileStore.Upload(ctx, cmd.File)
        if err != nil {
            return err
        }

        // Store file reference
        return s.store.AttachFile(ctx, entity.ID, filePath)
    })

    // Cleanup on failure
    if err != nil && filePath != "" {
        _ = s.fileStore.Delete(context.Background(), filePath)
    }

    return err
}
```

## Error Propagation

### Wrapping Errors

Use error wrapping to preserve context:

```go
func (s *Service) CreateMaintenance(ctx context.Context, cmd *CreateCmd) (*Maintenance, error) {
    ctx = xlog.WithOperation(ctx, "service.Maint.Create")

    if err := validate(cmd); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    maint := buildMaintenance(cmd)

    err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
        if err := s.store.Create(ctx, maint); err != nil {
            return fmt.Errorf("create maintenance: %w", err)
        }

        if err := s.store.AddResources(ctx, maint.ID, maint.Resources); err != nil {
            return fmt.Errorf("add resources: %w", err)
        }

        return nil
    })

    if err != nil {
        return nil, fmt.Errorf("transaction failed: %w", err)
    }

    return maint, nil
}
```

### Error Translation

Convert database errors to domain errors:

```go
func translateDBError(err error) error {
    if err == nil {
        return nil
    }

    // Not found
    if errors.Is(err, qrm.ErrNoRows) {
        return apperr.ErrMaintNotFound
    }

    // Parse constraint violations
    if isConstraintViolation(err) {
        constraint := parseConstraintError(err)

        switch constraint.Constraint {
        case "maintenances_pkey":
            return apperr.ErrDuplicateID
        case "maintenances_unique_title":
            return apperr.ErrDuplicateTitle
        case "fk_maintenance_resource":
            return apperr.ErrInvalidResource
        default:
            return fmt.Errorf("constraint violation: %s: %w", constraint.Constraint, err)
        }
    }

    // Transaction conflicts
    if isSerializationFailure(err) {
        return apperr.ErrConcurrentModification
    }

    if isDeadlock(err) {
        return apperr.ErrDeadlock
    }

    // Unknown error
    return err
}

// Usage in store
func (s *Store) Create(ctx context.Context, maint *entity.Maintenance) error {
    stmt := table.Maintenances.INSERT(/*...*/)

    _, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
    return translateDBError(err)
}
```

### Error Context

Add context to errors for debugging:

```go
type ErrorContext struct {
    Operation string
    EntityID  uuid.UUID
    Details   map[string]interface{}
}

func (e *ErrorContext) Error() string {
    return fmt.Sprintf("operation %s failed for entity %s", e.Operation, e.EntityID)
}

func wrapWithContext(err error, ctx ErrorContext) error {
    if err == nil {
        return nil
    }
    return fmt.Errorf("%w: %v", err, ctx)
}

// Usage
func (s *Service) Update(ctx context.Context, id uuid.UUID, update *Update) error {
    err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
        // ... transaction operations
    })

    if err != nil {
        return wrapWithContext(err, ErrorContext{
            Operation: "Update",
            EntityID:  id,
            Details: map[string]interface{}{
                "status": update.Status,
                "fields": update.ModifiedFields,
            },
        })
    }

    return nil
}
```

## Partial Success Handling

### Idempotent Operations

Make operations safe to retry:

```go
func (s *Service) CreateMaintenanceIdempotent(ctx context.Context, idempotencyKey string, cmd *CreateCmd) (*Maintenance, error) {
    // Check if already created
    existing, err := s.store.FindByIdempotencyKey(ctx, idempotencyKey)
    if err == nil {
        xlog.Info(ctx, "maintenance already created, returning existing",
            zap.String("key", idempotencyKey),
            zap.String("id", existing.ID.String()))
        return existing, nil
    }

    if !errors.Is(err, apperr.ErrMaintNotFound) {
        return nil, err
    }

    // Create new
    maint := buildMaintenance(cmd)
    maint.IdempotencyKey = idempotencyKey

    err = s.txManager.WithinTx(ctx, func(ctx context.Context) error {
        return s.store.Create(ctx, maint)
    })

    if err != nil {
        // Check if race condition (another request created it)
        if isUniqueViolation(err, "idx_idempotency_key") {
            xlog.Warn(ctx, "race condition on idempotency key",
                zap.String("key", idempotencyKey))

            // Fetch the one that won the race
            return s.store.FindByIdempotencyKey(ctx, idempotencyKey)
        }

        return nil, err
    }

    return maint, nil
}
```

### Batch Processing with Partial Failure

```go
type BatchResult struct {
    Succeeded []uuid.UUID
    Failed    map[uuid.UUID]error
}

func (s *Service) BatchUpdate(ctx context.Context, updates []Update) (*BatchResult, error) {
    result := &BatchResult{
        Succeeded: make([]uuid.UUID, 0),
        Failed:    make(map[uuid.UUID]error),
    }

    // Process each in separate transaction
    for _, update := range updates {
        err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
            return s.performUpdate(ctx, update)
        })

        if err != nil {
            result.Failed[update.ID] = err
            xlog.Error(ctx, "update failed",
                zap.String("id", update.ID.String()),
                zap.Error(err))
        } else {
            result.Succeeded = append(result.Succeeded, update.ID)
        }
    }

    // Return error only if all failed
    if len(result.Failed) == len(updates) {
        return result, errors.New("all updates failed")
    }

    return result, nil
}
```

### Compensating Transactions

For distributed operations:

```go
func (s *Service) CreateWithExternalAPI(ctx context.Context, cmd *CreateCmd) (*Entity, error) {
    var externalID string

    // Create in database
    entity, err := s.createEntity(ctx, cmd)
    if err != nil {
        return nil, err
    }

    // Call external API
    externalID, err = s.externalAPI.Create(ctx, entity)
    if err != nil {
        // Compensate: delete from database
        xlog.Error(ctx, "external API failed, compensating",
            zap.String("entity_id", entity.ID.String()),
            zap.Error(err))

        if delErr := s.deleteEntity(ctx, entity.ID); delErr != nil {
            xlog.Error(ctx, "compensation failed",
                zap.String("entity_id", entity.ID.String()),
                zap.Error(delErr))
        }

        return nil, fmt.Errorf("failed to create in external system: %w", err)
    }

    // Store external ID
    err = s.txManager.WithinTx(ctx, func(ctx context.Context) error {
        return s.store.SetExternalID(ctx, entity.ID, externalID)
    })

    if err != nil {
        // Compensate: delete from external API
        xlog.Error(ctx, "failed to store external ID, compensating", zap.Error(err))

        if delErr := s.externalAPI.Delete(ctx, externalID); delErr != nil {
            xlog.Error(ctx, "compensation failed, manual cleanup required",
                zap.String("external_id", externalID),
                zap.Error(delErr))
        }

        return nil, err
    }

    return entity, nil
}
```

## Recovery Patterns

### Retry with Backoff

```go
type RetryConfig struct {
    MaxAttempts int
    InitialBackoff time.Duration
    MaxBackoff time.Duration
    BackoffMultiplier float64
}

func (s *Service) ExecuteWithRetry(ctx context.Context, cfg RetryConfig, fn func(context.Context) error) error {
    var lastErr error

    for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
        err := fn(ctx)
        if err == nil {
            return nil
        }

        lastErr = err

        // Don't retry on business errors
        if isBusinessError(err) || isValidationError(err) {
            return err
        }

        // Don't retry if no more attempts
        if attempt == cfg.MaxAttempts-1 {
            break
        }

        // Calculate backoff
        backoff := time.Duration(float64(cfg.InitialBackoff) *
            math.Pow(cfg.BackoffMultiplier, float64(attempt)))
        if backoff > cfg.MaxBackoff {
            backoff = cfg.MaxBackoff
        }

        xlog.Warn(ctx, "operation failed, retrying",
            zap.Int("attempt", attempt+1),
            zap.Duration("backoff", backoff),
            zap.Error(err))

        select {
        case <-ctx.Done():
            return fmt.Errorf("context cancelled: %w", ctx.Err())
        case <-time.After(backoff):
        }
    }

    return fmt.Errorf("failed after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

// Usage
func (s *Service) Update(ctx context.Context, id uuid.UUID, update *Update) error {
    return s.ExecuteWithRetry(ctx, RetryConfig{
        MaxAttempts:       3,
        InitialBackoff:    100 * time.Millisecond,
        MaxBackoff:        2 * time.Second,
        BackoffMultiplier: 2.0,
    }, func(ctx context.Context) error {
        return s.txManager.WithinTx(ctx, func(ctx context.Context) error {
            return s.performUpdate(ctx, id, update)
        })
    })
}
```

### Circuit Breaker

Prevent cascading failures:

```go
type CircuitBreaker struct {
    mu sync.Mutex

    maxFailures int
    resetTimeout time.Duration

    failures int
    lastFailureTime time.Time
    state CircuitState
}

type CircuitState int

const (
    CircuitClosed CircuitState = iota  // Normal operation
    CircuitOpen                         // Rejecting requests
    CircuitHalfOpen                     // Testing if service recovered
)

func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
    cb.mu.Lock()

    // Check state
    switch cb.state {
    case CircuitOpen:
        // Check if we should try again
        if time.Since(cb.lastFailureTime) > cb.resetTimeout {
            cb.state = CircuitHalfOpen
            cb.mu.Unlock()
        } else {
            cb.mu.Unlock()
            return errors.New("circuit breaker open")
        }
    case CircuitHalfOpen:
        cb.mu.Unlock()
    case CircuitClosed:
        cb.mu.Unlock()
    }

    // Execute
    err := fn(ctx)

    cb.mu.Lock()
    defer cb.mu.Unlock()

    if err != nil {
        cb.failures++
        cb.lastFailureTime = time.Now()

        if cb.failures >= cb.maxFailures {
            cb.state = CircuitOpen
        }

        return err
    }

    // Success - reset
    cb.failures = 0
    cb.state = CircuitClosed
    return nil
}
```

### Saga Pattern

For complex multi-step operations:

```go
type SagaStep struct {
    Name string
    Do   func(context.Context) error
    Undo func(context.Context) error
}

type Saga struct {
    steps []SagaStep
}

func (s *Saga) Execute(ctx context.Context) error {
    executed := make([]int, 0)

    // Execute steps
    for i, step := range s.steps {
        xlog.Info(ctx, "executing saga step", zap.String("step", step.Name))

        if err := step.Do(ctx); err != nil {
            xlog.Error(ctx, "saga step failed, rolling back",
                zap.String("step", step.Name),
                zap.Error(err))

            // Rollback in reverse order
            for j := len(executed) - 1; j >= 0; j-- {
                undoStep := s.steps[executed[j]]
                xlog.Info(ctx, "undoing saga step", zap.String("step", undoStep.Name))

                if undoErr := undoStep.Undo(ctx); undoErr != nil {
                    xlog.Error(ctx, "undo failed",
                        zap.String("step", undoStep.Name),
                        zap.Error(undoErr))
                }
            }

            return fmt.Errorf("saga failed at step %s: %w", step.Name, err)
        }

        executed = append(executed, i)
    }

    return nil
}

// Usage
func (s *Service) ComplexOperation(ctx context.Context) error {
    saga := &Saga{
        steps: []SagaStep{
            {
                Name: "create_maintenance",
                Do: func(ctx context.Context) error {
                    return s.createMaintenance(ctx)
                },
                Undo: func(ctx context.Context) error {
                    return s.deleteMaintenance(ctx)
                },
            },
            {
                Name: "reserve_resources",
                Do: func(ctx context.Context) error {
                    return s.reserveResources(ctx)
                },
                Undo: func(ctx context.Context) error {
                    return s.releaseResources(ctx)
                },
            },
            {
                Name: "send_notifications",
                Do: func(ctx context.Context) error {
                    return s.sendNotifications(ctx)
                },
                Undo: func(ctx context.Context) error {
                    return s.cancelNotifications(ctx)
                },
            },
        },
    }

    return saga.Execute(ctx)
}
```

## Best Practices

1. **Return Errors Directly**: Let TxManager handle rollback
2. **Wrap Errors**: Preserve error chain for debugging
3. **Translate Database Errors**: Convert to domain errors at boundary
4. **Log Rollbacks**: Always log why rollback occurred
5. **Distinguish Error Types**: Business vs technical vs validation
6. **Implement Retry Logic**: For transient errors only
7. **Use Idempotency**: Make operations safe to retry
8. **Clean Up Side Effects**: Delete files, cancel API calls, etc.
9. **Monitor Error Rates**: Track transaction failure patterns
10. **Document Recovery**: Clear process for manual recovery
