# Unit of Work Pattern

Advanced pattern for managing complex transactional workflows with change tracking and deferred operations.

## Table of Contents

1. [Pattern Overview](#pattern-overview)
2. [When to Use](#when-to-use)
3. [Implementation](#implementation)
4. [Common Use Cases](#common-use-cases)
5. [vs Simple Transactions](#vs-simple-transactions)

## Pattern Overview

The Unit of Work pattern tracks changes during a business transaction and coordinates writing out changes as a single operation.

### Core Concepts

**Change Tracking**: Track all modifications before persisting
**Deferred Execution**: Execute all changes at once during commit
**Atomic Updates**: All changes succeed or all fail
**Domain Focus**: Business logic doesn't manage transaction details

### Architecture

```
┌─────────────────────────────────────────────────────┐
│              Business Transaction                   │
│  ┌─────────────────────────────────────────────┐   │
│  │         Unit of Work                        │   │
│  │                                             │   │
│  │  New:      [Entity1, Entity2]              │   │
│  │  Modified: [Entity3, Entity4]              │   │
│  │  Deleted:  [Entity5]                       │   │
│  │                                             │   │
│  └──────────────────┬──────────────────────────┘   │
│                     │                              │
│                     ▼                              │
│              Commit() calls                        │
│                     │                              │
│     ┌───────────────┼───────────────┐             │
│     │               │               │             │
│     ▼               ▼               ▼             │
│  INSERT         UPDATE          DELETE            │
│  (New)       (Modified)        (Deleted)          │
└─────────────────────────────────────────────────────┘
```

## When to Use

### Good Fit

**Complex Domain Operations**:
```go
// Order processing with multiple entities
func (s *Service) ProcessOrder(ctx context.Context, order *Order) error {
    uow := NewUnitOfWork(s.txManager)

    // Track all changes
    uow.RegisterNew(order)
    uow.RegisterNew(order.Invoice)
    for _, item := range order.Items {
        uow.RegisterNew(item)

        // Update inventory
        inventory := s.inventoryStore.Get(ctx, item.ProductID)
        inventory.Quantity -= item.Quantity
        uow.RegisterModified(inventory)
    }

    // Commit all at once
    return uow.Commit(ctx)
}
```

**Cascading Updates**:
```go
// Update entity and all related entities
func (s *Service) UpdateMaintenanceScope(ctx context.Context, id uuid.UUID, scope Scope) error {
    uow := NewUnitOfWork(s.txManager)

    maint := s.store.Get(ctx, id)
    maint.Scope = scope
    uow.RegisterModified(maint)

    // Cascade to resources
    if scope == ScopeAll {
        resources := s.resourceStore.GetAll(ctx)
        for _, r := range resources {
            assignment := NewAssignment(maint.ID, r.ID)
            uow.RegisterNew(assignment)
        }
    }

    return uow.Commit(ctx)
}
```

**Batch Operations**:
```go
// Process multiple entities with shared logic
func (s *Service) BulkUpdate(ctx context.Context, ids []uuid.UUID) error {
    uow := NewUnitOfWork(s.txManager)

    for _, id := range ids {
        entity := s.store.Get(ctx, id)
        if shouldUpdate(entity) {
            entity.Status = StatusProcessed
            uow.RegisterModified(entity)

            // Create audit log
            log := NewAuditLog(entity)
            uow.RegisterNew(log)
        }
    }

    return uow.Commit(ctx)
}
```

### Not a Good Fit

**Simple Operations**: Use direct TxManager for simple create/update/delete

**Read-Only Operations**: No changes to track

**Single Entity Updates**: Overhead not justified

**Streaming Operations**: Can't accumulate all changes in memory

## Implementation

### Basic Unit of Work

```go
package uow

import (
    "context"
    "fmt"

    "github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Entity that can be tracked
type Entity interface {
    GetID() string
    GetTableName() string
}

// Repository interface for persisting entities
type Repository interface {
    Insert(ctx context.Context, entity Entity) error
    Update(ctx context.Context, entity Entity) error
    Delete(ctx context.Context, entity Entity) error
}

// UnitOfWork tracks changes and coordinates persistence
type UnitOfWork struct {
    txManager *dbtx.TxManager
    repos     map[string]Repository

    newEntities      []Entity
    modifiedEntities []Entity
    deletedEntities  []Entity
}

func NewUnitOfWork(txManager *dbtx.TxManager) *UnitOfWork {
    return &UnitOfWork{
        txManager:        txManager,
        repos:            make(map[string]Repository),
        newEntities:      make([]Entity, 0),
        modifiedEntities: make([]Entity, 0),
        deletedEntities:  make([]Entity, 0),
    }
}

// Register repositories for each entity type
func (u *UnitOfWork) RegisterRepository(tableName string, repo Repository) {
    u.repos[tableName] = repo
}

// Track new entity for insertion
func (u *UnitOfWork) RegisterNew(entity Entity) {
    u.newEntities = append(u.newEntities, entity)
}

// Track modified entity for update
func (u *UnitOfWork) RegisterModified(entity Entity) {
    u.modifiedEntities = append(u.modifiedEntities, entity)
}

// Track entity for deletion
func (u *UnitOfWork) RegisterDeleted(entity Entity) {
    u.deletedEntities = append(u.deletedEntities, entity)
}

// Commit all changes in a transaction
func (u *UnitOfWork) Commit(ctx context.Context) error {
    return u.txManager.WithinTx(ctx, func(ctx context.Context) error {
        // Insert new entities
        for _, entity := range u.newEntities {
            repo, ok := u.repos[entity.GetTableName()]
            if !ok {
                return fmt.Errorf("no repository for table: %s", entity.GetTableName())
            }
            if err := repo.Insert(ctx, entity); err != nil {
                return fmt.Errorf("insert failed: %w", err)
            }
        }

        // Update modified entities
        for _, entity := range u.modifiedEntities {
            repo, ok := u.repos[entity.GetTableName()]
            if !ok {
                return fmt.Errorf("no repository for table: %s", entity.GetTableName())
            }
            if err := repo.Update(ctx, entity); err != nil {
                return fmt.Errorf("update failed: %w", err)
            }
        }

        // Delete entities
        for _, entity := range u.deletedEntities {
            repo, ok := u.repos[entity.GetTableName()]
            if !ok {
                return fmt.Errorf("no repository for table: %s", entity.GetTableName())
            }
            if err := repo.Delete(ctx, entity); err != nil {
                return fmt.Errorf("delete failed: %w", err)
            }
        }

        return nil
    })
}

// Rollback clears all tracked changes
func (u *UnitOfWork) Rollback() {
    u.newEntities = u.newEntities[:0]
    u.modifiedEntities = u.modifiedEntities[:0]
    u.deletedEntities = u.deletedEntities[:0]
}
```

### Enhanced Unit of Work with Features

```go
type EnhancedUnitOfWork struct {
    *UnitOfWork

    // Deferred operations
    deferredOps []func(context.Context) error

    // Event tracking
    events []DomainEvent

    // Validation
    validators []func() error
}

func NewEnhancedUnitOfWork(txManager *dbtx.TxManager) *EnhancedUnitOfWork {
    return &EnhancedUnitOfWork{
        UnitOfWork:  NewUnitOfWork(txManager),
        deferredOps: make([]func(context.Context) error, 0),
        events:      make([]DomainEvent, 0),
        validators:  make([]func() error, 0),
    }
}

// Defer operation until commit
func (u *EnhancedUnitOfWork) Defer(op func(context.Context) error) {
    u.deferredOps = append(u.deferredOps, op)
}

// Track domain event
func (u *EnhancedUnitOfWork) PublishEvent(event DomainEvent) {
    u.events = append(u.events, event)
}

// Add validation to run before commit
func (u *EnhancedUnitOfWork) AddValidation(validator func() error) {
    u.validators = append(u.validators, validator)
}

// Commit with validation and deferred operations
func (u *EnhancedUnitOfWork) Commit(ctx context.Context) error {
    // Run validations first
    for _, validator := range u.validators {
        if err := validator(); err != nil {
            return fmt.Errorf("validation failed: %w", err)
        }
    }

    return u.txManager.WithinTx(ctx, func(ctx context.Context) error {
        // Persist entity changes
        if err := u.UnitOfWork.Commit(ctx); err != nil {
            return err
        }

        // Execute deferred operations
        for _, op := range u.deferredOps {
            if err := op(ctx); err != nil {
                return fmt.Errorf("deferred operation failed: %w", err)
            }
        }

        // Publish events after successful commit
        for _, event := range u.events {
            if err := u.eventPublisher.Publish(ctx, event); err != nil {
                return fmt.Errorf("event publish failed: %w", err)
            }
        }

        return nil
    })
}
```

## Common Use Cases

### Use Case 1: Complex Order Processing

```go
func (s *OrderService) CreateOrder(ctx context.Context, cmd *CreateOrderCmd) (*Order, error) {
    uow := NewEnhancedUnitOfWork(s.txManager)

    // Register repositories
    uow.RegisterRepository("orders", s.orderRepo)
    uow.RegisterRepository("order_items", s.itemRepo)
    uow.RegisterRepository("inventory", s.inventoryRepo)
    uow.RegisterRepository("audit_logs", s.auditRepo)

    // Create order
    order := &Order{
        ID:         uuid.New(),
        CustomerID: cmd.CustomerID,
        Status:     OrderStatusPending,
        CreatedAt:  time.Now(),
    }
    uow.RegisterNew(order)

    // Process items
    for _, itemCmd := range cmd.Items {
        // Create order item
        item := &OrderItem{
            ID:        uuid.New(),
            OrderID:   order.ID,
            ProductID: itemCmd.ProductID,
            Quantity:  itemCmd.Quantity,
            Price:     itemCmd.Price,
        }
        uow.RegisterNew(item)

        // Update inventory
        inventory, err := s.inventoryRepo.Get(ctx, itemCmd.ProductID)
        if err != nil {
            return nil, err
        }

        if inventory.Quantity < itemCmd.Quantity {
            return nil, ErrInsufficientInventory
        }

        inventory.Quantity -= itemCmd.Quantity
        inventory.Reserved += itemCmd.Quantity
        uow.RegisterModified(inventory)
    }

    // Add audit log
    auditLog := &AuditLog{
        EntityType: "order",
        EntityID:   order.ID,
        Action:     "created",
        Timestamp:  time.Now(),
    }
    uow.RegisterNew(auditLog)

    // Publish domain event
    uow.PublishEvent(OrderCreatedEvent{OrderID: order.ID})

    // Commit all changes
    if err := uow.Commit(ctx); err != nil {
        return nil, err
    }

    return order, nil
}
```

### Use Case 2: Cascade Delete

```go
func (s *MaintenanceService) DeleteMaintenance(ctx context.Context, id uuid.UUID) error {
    uow := NewUnitOfWork(s.txManager)

    uow.RegisterRepository("maintenances", s.maintRepo)
    uow.RegisterRepository("resource_assignments", s.assignmentRepo)
    uow.RegisterRepository("schedules", s.scheduleRepo)
    uow.RegisterRepository("notifications", s.notificationRepo)

    // Get maintenance
    maint, err := s.maintRepo.Get(ctx, id)
    if err != nil {
        return err
    }
    uow.RegisterDeleted(maint)

    // Delete all resource assignments
    assignments, err := s.assignmentRepo.FindByMaintenance(ctx, id)
    if err != nil {
        return err
    }
    for _, assignment := range assignments {
        uow.RegisterDeleted(assignment)
    }

    // Delete schedules
    schedules, err := s.scheduleRepo.FindByMaintenance(ctx, id)
    if err != nil {
        return err
    }
    for _, schedule := range schedules {
        uow.RegisterDeleted(schedule)
    }

    // Delete notifications
    notifications, err := s.notificationRepo.FindByMaintenance(ctx, id)
    if err != nil {
        return err
    }
    for _, notification := range notifications {
        uow.RegisterDeleted(notification)
    }

    return uow.Commit(ctx)
}
```

### Use Case 3: Deferred Operations

```go
func (s *Service) UpdateWithNotification(ctx context.Context, id uuid.UUID, update *Update) error {
    uow := NewEnhancedUnitOfWork(s.txManager)
    uow.RegisterRepository("entities", s.repo)

    entity, err := s.repo.Get(ctx, id)
    if err != nil {
        return err
    }

    entity.ApplyUpdate(update)
    uow.RegisterModified(entity)

    // Defer email until after commit
    uow.Defer(func(ctx context.Context) error {
        return s.emailService.SendUpdateNotification(ctx, entity)
    })

    // Defer cache invalidation
    uow.Defer(func(ctx context.Context) error {
        return s.cache.Delete(ctx, entity.CacheKey())
    })

    return uow.Commit(ctx)
}
```

## vs Simple Transactions

### When Unit of Work is Better

| Scenario | Simple Transaction | Unit of Work |
|----------|-------------------|--------------|
| Multiple entity types | Manual coordination | Automatic tracking |
| Cascading updates | Must code explicitly | Declarative registration |
| Validation | Ad-hoc | Centralized validators |
| Events | Manual timing | Automatic post-commit |
| Testing | Mock each operation | Mock repository layer |

### When Simple Transaction is Better

| Aspect | Reason |
|--------|--------|
| Simple CRUD | Overhead not justified |
| Single entity | No coordination needed |
| Streaming data | Can't buffer all changes |
| Performance critical | UoW adds abstraction cost |

### Decision Matrix

```
Start here → Does operation involve multiple entities?
                 ├─ No → Are there cascading effects?
                 │         ├─ No → Use Simple Transaction
                 │         └─ Yes → Consider Unit of Work
                 └─ Yes → Do entities have complex relationships?
                             ├─ No → Use Simple Transaction
                             └─ Yes → Use Unit of Work
```

### Code Comparison

**Simple Transaction:**

```go
err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
    if err := s.store1.Create(ctx, entity1); err != nil {
        return err
    }
    if err := s.store2.Create(ctx, entity2); err != nil {
        return err
    }
    return nil
})
```

**Pros:**
- Straightforward
- Less code
- Direct control
- Lower overhead

**Unit of Work:**

```go
uow := NewUnitOfWork(s.txManager)
uow.RegisterRepository("table1", s.store1)
uow.RegisterRepository("table2", s.store2)
uow.RegisterNew(entity1)
uow.RegisterNew(entity2)
err := uow.Commit(ctx)
```

**Pros:**
- Declarative
- Testable
- Extensible
- Separation of concerns

## Best Practices

1. **Clear Scope**: Define what entities are in scope before starting
2. **Fail Fast**: Validate inputs before creating UoW
3. **Repository Per Table**: Don't mix entity types in repositories
4. **Explicit Registration**: Register repositories explicitly
5. **Rollback on Failure**: Clear tracked changes if commit fails
6. **Test Isolation**: Each test should create new UoW instance
7. **Document Lifecycle**: Make entity state transitions clear
8. **Avoid Lazy Loading**: Load all needed entities upfront
