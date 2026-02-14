# Idempotency Patterns

Making operations safely repeatable to handle retries, network failures, and duplicate requests.

## Table of Contents

1. [Why Idempotency Matters](#why-idempotency-matters)
2. [Idempotency Keys](#idempotency-keys)
3. [Database-Level Strategies](#database-level-strategies)
4. [Application-Level Strategies](#application-level-strategies)
5. [Testing Idempotency](#testing-idempotency)

## Why Idempotency Matters

### Problem Scenarios

**Network Retry**:
```
Client                      Server
  |                            |
  |------- POST /orders ------>|
  |                            | Creates order
  |<------ 200 OK --------X    | Response lost
  |                            |
  |------- POST /orders ------>| Duplicate!
```

**Client Retry**:
```
Client                      Server
  |                            |
  |------- POST /orders ------>|
  |<------ 500 Error ----------| Server crashes
  |                            |
  |------- POST /orders ------>| Retry creates duplicate
```

**Multiple Clicks**:
```
User double-clicks submit button
  → Two requests sent
  → Two orders created
  → User charged twice
```

### What is Idempotency

An operation is idempotent if executing it multiple times has the same effect as executing it once.

**Naturally Idempotent**:
- `GET /orders/123` - Read is always idempotent
- `DELETE /orders/123` - Already deleted = still deleted
- `PUT /orders/123` - Full replacement is idempotent

**Not Naturally Idempotent**:
- `POST /orders` - Creates new order each time
- `PATCH /orders/123 { increment: 1 }` - Changes value each time
- `POST /orders/123/notifications` - Sends notification each time

## Idempotency Keys

### Implementation

**Database Schema**:
```sql
CREATE TABLE maintenances (
    id UUID PRIMARY KEY,
    title TEXT NOT NULL,
    status TEXT NOT NULL,
    idempotency_key TEXT,
    created_at TIMESTAMP NOT NULL,
    CONSTRAINT idx_maintenances_idempotency_key UNIQUE (idempotency_key)
);

CREATE INDEX idx_maintenances_idempotency_key ON maintenances (idempotency_key)
    WHERE idempotency_key IS NOT NULL;
```

**Entity**:
```go
type Maintenance struct {
    ID             uuid.UUID
    Title          string
    Status         Status
    IdempotencyKey *string  // Optional, only for create operations
    CreatedAt      time.Time
}
```

**Service Implementation**:
```go
func (s *Service) CreateMaintenanceIdempotent(
    ctx context.Context,
    idempotencyKey string,
    cmd *CreateMaintenanceCmd,
) (*Maintenance, error) {
    ctx = xlog.WithOperation(ctx, "service.Maint.CreateIdempotent")

    // Validate idempotency key
    if idempotencyKey == "" {
        return nil, errors.New("idempotency key required")
    }

    // Check if already created
    existing, err := s.store.FindByIdempotencyKey(ctx, idempotencyKey)
    if err == nil {
        xlog.Info(ctx, "idempotent request, returning existing maintenance",
            zap.String("key", idempotencyKey),
            zap.String("id", existing.ID.String()))
        return existing, nil
    }

    if !errors.Is(err, apperr.ErrNotFound) {
        return nil, fmt.Errorf("lookup by idempotency key: %w", err)
    }

    // Validate command
    if err := validate(cmd); err != nil {
        return nil, err
    }

    // Create new maintenance
    maint := &Maintenance{
        ID:             xuuid.New(),
        Title:          cmd.Title,
        Status:         StatusDraft,
        IdempotencyKey: &idempotencyKey,
        CreatedAt:      time.Now(),
    }

    err = s.txManager.WithinTx(ctx, func(ctx context.Context) error {
        return s.store.Create(ctx, maint)
    })

    if err != nil {
        // Handle race condition
        if isUniqueViolation(err, "idx_maintenances_idempotency_key") {
            xlog.Warn(ctx, "race condition on idempotency key, fetching winner",
                zap.String("key", idempotencyKey))

            // Another request won the race, return their result
            return s.store.FindByIdempotencyKey(ctx, idempotencyKey)
        }

        return nil, fmt.Errorf("create maintenance: %w", err)
    }

    return maint, nil
}
```

**Store Implementation**:
```go
func (s *Store) FindByIdempotencyKey(ctx context.Context, key string) (*entity.Maintenance, error) {
    stmt := table.Maintenances.
        SELECT(table.Maintenances.AllColumns).
        WHERE(table.Maintenances.IdempotencyKey.EQ(postgres.String(key)))

    maint := new(model.Maintenances)
    err := stmt.QueryContext(ctx, s.db.Executor(ctx), maint)
    if err != nil {
        if errors.Is(err, qrm.ErrNoRows) {
            return nil, apperr.ErrNotFound
        }
        return nil, err
    }

    return fromDBMaintenance(maint), nil
}
```

### Idempotency Key Generation

**Client-Generated (Recommended)**:
```go
// Client generates UUID
idempotencyKey := uuid.New().String()

maintenance, err := client.CreateMaintenance(ctx, idempotencyKey, cmd)
```

**Request-ID Based**:
```go
// Use request ID from HTTP header
idempotencyKey := r.Header.Get("X-Request-ID")
if idempotencyKey == "" {
    return errors.New("X-Request-ID header required")
}
```

**Hash-Based**:
```go
// Hash request content (less flexible)
func generateIdempotencyKey(cmd *CreateMaintenanceCmd) string {
    h := sha256.New()
    h.Write([]byte(cmd.Title))
    h.Write([]byte(cmd.Description))
    // ... hash all relevant fields
    return hex.EncodeToString(h.Sum(nil))
}
```

### Key Expiration

For short-term deduplication:

```sql
CREATE TABLE idempotency_keys (
    key TEXT PRIMARY KEY,
    result JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_idempotency_keys_expires ON idempotency_keys (expires_at);

-- Cleanup job
DELETE FROM idempotency_keys WHERE expires_at < NOW();
```

```go
type IdempotencyStore struct {
    db *dbtx.DB
}

func (s *IdempotencyStore) Store(ctx context.Context, key string, result interface{}, ttl time.Duration) error {
    resultJSON, err := json.Marshal(result)
    if err != nil {
        return err
    }

    stmt := table.IdempotencyKeys.
        INSERT(
            table.IdempotencyKeys.Key,
            table.IdempotencyKeys.Result,
            table.IdempotencyKeys.ExpiresAt,
        ).
        VALUES(
            postgres.String(key),
            postgres.Raw("?", resultJSON),
            postgres.TimestampT(time.Now().Add(ttl)),
        )

    _, err = stmt.ExecContext(ctx, s.db.Executor(ctx))
    return err
}

func (s *IdempotencyStore) Get(ctx context.Context, key string, result interface{}) error {
    stmt := table.IdempotencyKeys.
        SELECT(table.IdempotencyKeys.Result).
        WHERE(
            table.IdempotencyKeys.Key.EQ(postgres.String(key)).
            AND(table.IdempotencyKeys.ExpiresAt.GT(postgres.TimestampT(time.Now()))),
        )

    var resultJSON []byte
    err := stmt.QueryContext(ctx, s.db.Executor(ctx), &resultJSON)
    if err != nil {
        if errors.Is(err, qrm.ErrNoRows) {
            return apperr.ErrNotFound
        }
        return err
    }

    return json.Unmarshal(resultJSON, result)
}
```

## Database-Level Strategies

### Unique Constraints

**Single Column**:
```sql
CREATE TABLE maintenances (
    id UUID PRIMARY KEY,
    external_id TEXT UNIQUE,  -- Prevent duplicate external IDs
    title TEXT NOT NULL
);
```

**Composite Unique Constraint**:
```sql
CREATE TABLE resource_assignments (
    id UUID PRIMARY KEY,
    maintenance_id UUID NOT NULL,
    resource_id UUID NOT NULL,
    CONSTRAINT unique_assignment UNIQUE (maintenance_id, resource_id)
);
```

**Conditional Unique Constraint**:
```sql
-- Only one active maintenance per resource
CREATE UNIQUE INDEX idx_one_active_per_resource
ON resource_assignments (resource_id)
WHERE status = 'active';
```

### INSERT ... ON CONFLICT

**PostgreSQL Upsert**:
```go
func (s *Store) UpsertResourceAssignment(ctx context.Context, assignment *ResourceAssignment) error {
    stmt := `
        INSERT INTO resource_assignments (id, maintenance_id, resource_id, assigned_at)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (maintenance_id, resource_id)
        DO UPDATE SET
            assigned_at = EXCLUDED.assigned_at
        RETURNING id
    `

    err := s.db.Executor(ctx).QueryRowxContext(ctx, stmt,
        assignment.ID,
        assignment.MaintenanceID,
        assignment.ResourceID,
        assignment.AssignedAt,
    ).Scan(&assignment.ID)

    return err
}
```

**With Jet ORM**:
```go
func (s *Store) UpsertSettings(ctx context.Context, settings *Settings) error {
    stmt := table.Settings.
        INSERT(
            table.Settings.UserID,
            table.Settings.Key,
            table.Settings.Value,
        ).
        VALUES(
            postgres.UUID(settings.UserID),
            postgres.String(settings.Key),
            postgres.String(settings.Value),
        ).
        ON_CONFLICT(table.Settings.UserID, table.Settings.Key).
        DO_UPDATE(
            jet.SET(
                table.Settings.Value.SET(postgres.String(settings.Value)),
                table.Settings.UpdatedAt.SET(postgres.NOW()),
            ),
        )

    _, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
    return err
}
```

### Advisory Locks

For application-level mutual exclusion:

```go
func (s *Service) ProcessWithLock(ctx context.Context, resourceID uuid.UUID) error {
    // Convert UUID to int64 for advisory lock
    lockID := int64(binary.BigEndian.Uint64(resourceID[:8]))

    return s.txManager.WithinTx(ctx, func(ctx context.Context) error {
        // Acquire advisory lock
        var acquired bool
        err := s.db.Executor(ctx).QueryRowxContext(ctx,
            "SELECT pg_try_advisory_xact_lock($1)", lockID,
        ).Scan(&acquired)

        if err != nil {
            return err
        }

        if !acquired {
            return errors.New("resource locked by another process")
        }

        // Lock held for transaction duration
        return s.performProcessing(ctx, resourceID)
    })
}
```

**Advisory lock patterns**:
```sql
-- Transaction-level lock (released on commit/rollback)
SELECT pg_advisory_xact_lock(123);

-- Session-level lock (must be explicitly released)
SELECT pg_advisory_lock(123);
SELECT pg_advisory_unlock(123);

-- Try without blocking
SELECT pg_try_advisory_lock(123);
```

## Application-Level Strategies

### Deduplication Cache

**In-Memory Cache**:
```go
type DeduplicationCache struct {
    mu    sync.RWMutex
    cache map[string]*CacheEntry
}

type CacheEntry struct {
    Result    interface{}
    ExpiresAt time.Time
}

func (c *DeduplicationCache) GetOrExecute(
    ctx context.Context,
    key string,
    ttl time.Duration,
    fn func(context.Context) (interface{}, error),
) (interface{}, error) {
    // Check cache
    c.mu.RLock()
    entry, exists := c.cache[key]
    c.mu.RUnlock()

    if exists && time.Now().Before(entry.ExpiresAt) {
        return entry.Result, nil
    }

    // Execute
    result, err := fn(ctx)
    if err != nil {
        return nil, err
    }

    // Store in cache
    c.mu.Lock()
    c.cache[key] = &CacheEntry{
        Result:    result,
        ExpiresAt: time.Now().Add(ttl),
    }
    c.mu.Unlock()

    return result, nil
}
```

**Redis-Based**:
```go
func (s *Service) CreateWithRedisDedup(
    ctx context.Context,
    key string,
    cmd *CreateCmd,
) (*Entity, error) {
    // Try to set key with NX (only if not exists)
    processing := s.redis.SetNX(ctx, "processing:"+key, "1", 5*time.Minute)
    if !processing.Val() {
        // Another request is processing this key

        // Wait for result
        for i := 0; i < 30; i++ {
            resultJSON, err := s.redis.Get(ctx, "result:"+key).Result()
            if err == nil {
                var entity Entity
                json.Unmarshal([]byte(resultJSON), &entity)
                return &entity, nil
            }

            time.Sleep(1 * time.Second)
        }

        return nil, errors.New("timeout waiting for result")
    }

    // We won the race, process the request
    entity, err := s.create(ctx, cmd)
    if err != nil {
        s.redis.Del(ctx, "processing:"+key)
        return nil, err
    }

    // Store result
    resultJSON, _ := json.Marshal(entity)
    s.redis.Set(ctx, "result:"+key, resultJSON, 5*time.Minute)
    s.redis.Del(ctx, "processing:"+key)

    return entity, nil
}
```

### State Machine Guards

Ensure operations only execute in valid states:

```go
type Status string

const (
    StatusDraft     Status = "draft"
    StatusScheduled Status = "scheduled"
    StatusActive    Status = "active"
    StatusCompleted Status = "completed"
    StatusCancelled Status = "cancelled"
)

var validTransitions = map[Status][]Status{
    StatusDraft:     {StatusScheduled, StatusCancelled},
    StatusScheduled: {StatusActive, StatusCancelled},
    StatusActive:    {StatusCompleted, StatusCancelled},
    StatusCompleted: {},
    StatusCancelled: {},
}

func (m *Maintenance) CanTransitionTo(target Status) bool {
    allowed, exists := validTransitions[m.Status]
    if !exists {
        return false
    }

    for _, status := range allowed {
        if status == target {
            return true
        }
    }

    return false
}

func (s *Service) TransitionStatus(
    ctx context.Context,
    id uuid.UUID,
    target Status,
) error {
    return s.txManager.WithinTx(ctx, func(ctx context.Context) error {
        // Lock and load current state
        maint, err := s.store.GetForUpdate(ctx, id)
        if err != nil {
            return err
        }

        // Check if transition is valid
        if !maint.CanTransitionTo(target) {
            return fmt.Errorf("cannot transition from %s to %s",
                maint.Status, target)
        }

        // Already in target state = idempotent success
        if maint.Status == target {
            xlog.Info(ctx, "already in target status",
                zap.String("status", string(target)))
            return nil
        }

        // Transition
        maint.Status = target
        return s.store.Update(ctx, maint)
    })
}
```

### Versioning

Track entity versions to detect concurrent modifications:

```go
type VersionedEntity struct {
    ID      uuid.UUID
    Data    string
    Version int
}

func (s *Service) UpdateVersioned(
    ctx context.Context,
    id uuid.UUID,
    expectedVersion int,
    update *Update,
) error {
    return s.txManager.WithinTx(ctx, func(ctx context.Context) error {
        entity, err := s.store.Get(ctx, id)
        if err != nil {
            return err
        }

        // Check version
        if entity.Version != expectedVersion {
            return apperr.ErrVersionMismatch
        }

        // Apply update
        entity.ApplyUpdate(update)
        entity.Version++

        // Update with version check
        return s.store.UpdateWithVersion(ctx, entity, expectedVersion)
    })
}

func (s *Store) UpdateWithVersion(
    ctx context.Context,
    entity *VersionedEntity,
    expectedVersion int,
) error {
    stmt := table.Entities.
        UPDATE(
            table.Entities.Data,
            table.Entities.Version,
        ).
        SET(
            postgres.String(entity.Data),
            postgres.Int(entity.Version),
        ).
        WHERE(
            table.Entities.ID.EQ(postgres.UUID(entity.ID)).
            AND(table.Entities.Version.EQ(postgres.Int(expectedVersion))),
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
        return apperr.ErrVersionMismatch
    }

    return nil
}
```

## Testing Idempotency

### Test Pattern

```go
func TestCreateMaintenanceIdempotent(t *testing.T) {
    ctx := context.Background()
    service := setupService(t)

    idempotencyKey := uuid.New().String()
    cmd := &CreateMaintenanceCmd{
        Title:       "Test Maintenance",
        Description: "Test Description",
    }

    // First request
    maint1, err := service.CreateMaintenanceIdempotent(ctx, idempotencyKey, cmd)
    require.NoError(t, err)
    require.NotNil(t, maint1)

    // Second request with same key
    maint2, err := service.CreateMaintenanceIdempotent(ctx, idempotencyKey, cmd)
    require.NoError(t, err)
    require.NotNil(t, maint2)

    // Should return same entity
    assert.Equal(t, maint1.ID, maint2.ID)
    assert.Equal(t, maint1.Title, maint2.Title)

    // Should not create duplicate in database
    count, err := service.CountMaintenances(ctx)
    require.NoError(t, err)
    assert.Equal(t, 1, count)
}
```

### Concurrent Test

```go
func TestCreateMaintenanceConcurrentIdempotent(t *testing.T) {
    ctx := context.Background()
    service := setupService(t)

    idempotencyKey := uuid.New().String()
    cmd := &CreateMaintenanceCmd{
        Title:       "Test Maintenance",
        Description: "Test Description",
    }

    // Launch concurrent requests
    const concurrency = 10
    results := make(chan *Maintenance, concurrency)
    errors := make(chan error, concurrency)

    var wg sync.WaitGroup
    for i := 0; i < concurrency; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()

            maint, err := service.CreateMaintenanceIdempotent(ctx, idempotencyKey, cmd)
            if err != nil {
                errors <- err
                return
            }

            results <- maint
        }()
    }

    wg.Wait()
    close(results)
    close(errors)

    // Check no errors
    for err := range errors {
        t.Errorf("unexpected error: %v", err)
    }

    // All results should have same ID
    var firstID uuid.UUID
    for maint := range results {
        if firstID == uuid.Nil {
            firstID = maint.ID
        } else {
            assert.Equal(t, firstID, maint.ID)
        }
    }

    // Should only create one record
    count, err := service.CountMaintenances(ctx)
    require.NoError(t, err)
    assert.Equal(t, 1, count)
}
```

## Best Practices

1. **Require Idempotency Keys**: For all create/update APIs
2. **Client-Generated Keys**: Let clients generate unique keys
3. **Store Keys**: Keep idempotency keys in database
4. **Handle Race Conditions**: Use unique constraints + retry
5. **Set Expiration**: Clean up old keys (24-72 hours typical)
6. **Make State Transitions Idempotent**: Same transition twice = success
7. **Use Unique Constraints**: Natural keys prevent duplicates
8. **Document Behavior**: Clear docs on idempotency guarantees
9. **Test Concurrency**: Always test concurrent identical requests
10. **Monitor Duplicate Rate**: Track how often keys are reused
