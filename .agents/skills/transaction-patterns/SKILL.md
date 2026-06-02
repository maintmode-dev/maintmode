---
name: transaction-patterns
description: Transaction management patterns for Go with PostgreSQL, sqlx, and jet. Use when implementing transactional operations, handling database transactions with context, using TxManager pattern, implementing Unit of Work pattern, managing nested transactions, handling rollback strategies, implementing idempotency, configuring transaction isolation levels, handling transaction errors, or testing transactional code. Especially relevant for MaintMode project with its TxManager implementation in internal/utils/dbtx.
---

# Transaction Patterns for Go + PostgreSQL

Enterprise-grade transaction management patterns for Go applications using PostgreSQL with sqlx and jet ORM.

## Quick Start

### Basic TxManager Usage

The TxManager pattern provides context-based transaction management:

```go
func (s *Service) CreateDraft(ctx context.Context, cmd *entity.CreateMaintenanceCmd) (*entity.Maintenance, error) {
    // Validate before starting transaction
    if err := validate(cmd); err != nil {
        return nil, err
    }

    maint := buildEntity(cmd)

    // Use TxManager to wrap all operations
    err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
        if err := s.store.Create(ctx, maint); err != nil {
            return err
        }

        if len(maint.Resources) > 0 {
            if err := s.store.AddResources(ctx, maint.ID, maint.Resources); err != nil {
                return err
            }
        }

        return nil
    })

    return maint, err
}
```

### Store Pattern with Executor

Stores use the Executor pattern to work with or without transactions:

```go
type Store struct {
    db *dbtx.DB
}

func (s *Store) Create(ctx context.Context, item *entity.Item) error {
    stmt := table.Items.INSERT(/*...*/)

    // Executor automatically uses transaction if available in context
    _, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
    return err
}
```

## Core Concepts

### 1. TxManager Pattern

Central transaction coordinator that:
- Manages transaction lifecycle (begin, commit, rollback)
- Propagates transactions via context
- Handles panic recovery
- Provides consistent error handling

**Location in MaintMode:** `internal/utils/dbtx/tx_manager.go`

### 2. Context-Based Transactions

Transactions are stored in context and automatically discovered by stores:
- No need to pass transaction objects explicitly
- Type-safe with internal context key
- Works seamlessly with nested calls

**Location in MaintMode:** `internal/utils/dbtx/txctx.go`

### 3. Executor Pattern

Abstraction that returns either transaction or database connection:
- Stores don't need to know if they're in a transaction
- Single code path for transactional and non-transactional operations
- Simplifies testing

**Location in MaintMode:** `internal/utils/dbtx/executor.go`

## Common Patterns

### Pattern 1: Simple Transaction

For operations requiring multiple database calls:

```go
err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
    if err := s.store.Create(ctx, entity); err != nil {
        return err
    }
    if err := s.store.AddRelations(ctx, entity.ID, relations); err != nil {
        return err
    }
    return nil
})
```

**Key points:**
- Validate input before starting transaction
- Return errors directly - TxManager handles rollback
- All operations use the same context

### Pattern 2: Read-Modify-Write with Locking

For updates requiring consistency:

```go
err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
    // Lock row for update
    entity, err := s.store.GetForUpdate(ctx, id)
    if err != nil {
        return err
    }

    // Apply business logic
    if err := applyChanges(ctx, entity); err != nil {
        return err
    }

    // Save changes
    return s.store.Update(ctx, entity)
})
```

**Key points:**
- Use `FOR UPDATE` to prevent concurrent modifications
- Load, modify, save pattern prevents lost updates
- Business logic validates state before changes

### Pattern 3: Cross-Service Operations

For operations spanning multiple domain stores:

```go
err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
    maint, err := s.maintStore.GetForUpdate(ctx, maintID)
    if err != nil {
        return err
    }

    // Validate state transition
    if err := validateTransition(maint); err != nil {
        return err
    }

    // Update maintenance
    maint.Status = newStatus
    if err := s.maintStore.Update(ctx, maint); err != nil {
        return err
    }

    // Create audit log
    if err := s.auditStore.Create(ctx, buildAuditLog(maint)); err != nil {
        return err
    }

    return nil
})
```

**Key points:**
- Single transaction spans multiple stores
- All-or-nothing atomicity
- Consistent state across domain boundaries

## Detailed Guides

For comprehensive coverage of specific patterns, see reference files:

### [TxManager Implementation](references/txmanager.md)

Deep dive into the TxManager pattern:
- Complete implementation details
- Transaction lifecycle management
- Panic recovery and error handling
- Integration with sqlx

**Read when:** Implementing or modifying TxManager, understanding transaction mechanics, debugging transaction issues.

### [Unit of Work Pattern](references/unit-of-work.md)

Advanced pattern for tracking changes:
- Change tracking within transactions
- Deferred operations
- Composite operations
- When to use vs simple transactions

**Read when:** Implementing complex business transactions with multiple entities, building frameworks, handling deferred operations.

### [Isolation Levels & Concurrency](references/isolation-and-concurrency.md)

PostgreSQL-specific transaction behavior:
- Transaction isolation levels explained
- Optimistic vs pessimistic locking
- Handling concurrent modifications
- Serialization failures and retries

**Read when:** Dealing with high-concurrency scenarios, handling transaction conflicts, tuning performance, debugging race conditions.

### [Error Handling & Rollback](references/error-handling.md)

Robust error management in transactions:
- Error propagation strategies
- Rollback decision tree
- Partial success handling
- Error wrapping best practices

**Read when:** Implementing error handling, debugging rollback issues, designing recovery strategies.

### [Idempotency Patterns](references/idempotency.md)

Making operations safely repeatable:
- Idempotency keys
- Preventing duplicate operations
- Handling retries safely
- Database-level uniqueness constraints

**Read when:** Implementing APIs, handling retries, ensuring exactly-once semantics, designing resilient systems.

### [Testing Transactions](references/testing.md)

Comprehensive test strategies for transactional code:
- Testing service layer with transactions
- Testing store layer operations
- Transaction rollback testing
- Integration tests with real transactions
- Test isolation strategies (unique data, test transactions, cleanup)
- Concurrent transaction testing
- Common testing patterns and helpers

**Read when:** Writing tests for transactional services, setting up test infrastructure, testing concurrent operations, debugging test failures, implementing test isolation.

## Best Practices

1. **Validate Before Transaction** - Run validations before `WithinTx` to avoid unnecessary transaction overhead
2. **Keep Transactions Short** - Minimize transaction duration to reduce lock contention
3. **Use FOR UPDATE Sparingly** - Only lock rows when truly necessary to prevent conflicts
4. **Return Errors Directly** - Let TxManager handle rollback, don't try to rollback manually
5. **Avoid External Calls** - Don't make HTTP requests or other external calls within transactions
6. **Single Responsibility** - Each transaction should have one clear business purpose
7. **Test Both Paths** - Test both successful commit and rollback scenarios
8. **Log Context** - Include operation context in logs for traceability

## Anti-Patterns to Avoid

1. **Manual Transaction Management** - Don't use `db.Begin()` directly, use TxManager
2. **Long-Running Transactions** - Avoid business logic that takes significant time
3. **Nested WithinTx Calls** - Don't nest `WithinTx` calls (not currently supported)
4. **Swallowing Errors** - Always propagate errors to trigger rollback
5. **External State Changes** - Don't perform un-undoable side effects (emails,
   sync network sends) inside or before a commit. For queue work, use the
   transactional outbox instead (below) so the enqueue is part of the tx.
6. **Passing Transactions** - Never pass `*sqlx.Tx` directly, always use context
7. **Commit-then-enqueue** - Don't commit the DB write and then enqueue a task: a
   crash in between loses or orphans it. Enqueue inside the tx via the outbox.

## Transactional Outbox (queue writes)

When a transaction both changes state and must enqueue a goque task, the enqueue
joins the same tx so the two commit atomically. The bridge:

```go
if tx, ok := dbtx.TxFromContext(ctx); ok {
    ctx = goque.WithTx(ctx, tx) // the goque insert/cancel runs in the caller's tx
}
```

Go through `internal/services/messaging/scheduler` (which applies this bridge),
not goque directly. Enqueue inside the `WithinTx` callback and return any enqueue
error so the whole operation rolls back. See the `goque-async-patterns` skill for
payload, idempotency, and processor rules.

## Integration with Jet ORM

The transaction patterns work seamlessly with jet queries:

```go
err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
    // Jet query uses transaction automatically
    stmt := table.Items.
        SELECT(table.Items.AllColumns).
        WHERE(table.Items.ID.EQ(postgres.UUID(id))).
        FOR(postgres.UPDATE())

    item := new(model.Items)
    err := stmt.QueryContext(ctx, s.db.Executor(ctx), item)
    if err != nil {
        return err
    }

    // Update and save
    item.Status = newStatus
    updateStmt := table.Items.
        UPDATE(table.Items.Status).
        SET(postgres.String(newStatus)).
        WHERE(table.Items.ID.EQ(postgres.UUID(id)))

    _, err = updateStmt.ExecContext(ctx, s.db.Executor(ctx))
    return err
})
```

## Resources

- [PostgreSQL Transaction Documentation](https://www.postgresql.org/docs/current/tutorial-transactions.html)
- [sqlx Documentation](https://github.com/jmoiron/sqlx)
- [Jet ORM v2](https://github.com/go-jet/jet)
- [Go Database/SQL Tutorial](https://go.dev/doc/database/execute-transactions)
