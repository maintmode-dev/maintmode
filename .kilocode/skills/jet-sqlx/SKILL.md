---
name: jet-sqlx
description: Jet ORM v2 + sqlx for type-safe SQL queries, query builder, and data mapping with PostgreSQL. Use when working with Jet ORM v2, building type-safe queries, configuring query builder, mapping data between entities and database models, implementing transactions, or generating models from database schema.
license: MIT
metadata:
  category: development
  source:
    repository: project-specific
    path: jet-sqlx
---

# Jet ORM + sqlx

## Installation

### Install Jet v2

```bash
go get github.com/go-jet/jet/v2/postgres
go get github.com/go-jet/jet/v2/qrm
```

### Project Dependencies

```go
import (
    "github.com/go-jet/jet/v2/postgres"
    "github.com/go-jet/jet/v2/qrm"
    "github.com/jmoiron/sqlx"
)
```

## Quick Start

### 1. Generate Models

```bash
make db-models
```

### 2. Create Store

```go
type Store struct {
    db *dbtx.DB
}

func NewStore(db *sqlx.DB) *Store {
    return &Store{db: dbtx.NewDB(db)}
}
```

### 3. Simple SELECT Query

```go
func (s *Store) Get(ctx context.Context, id uuid.UUID) (*entity.Item, error) {
    stmt := table.Items.
        SELECT(table.Items.AllColumns).
        WHERE(table.Items.ID.EQ(postgres.UUID(id)))

    item := new(model.Items)
    err := stmt.QueryContext(ctx, s.db.Executor(ctx), item)
    if err != nil {
        if errors.Is(err, qrm.ErrNoRows) {
            return nil, apperr.ErrNotFound
        }
        return nil, err
    }

    return fromDBItem(item), nil
}
```

## Detailed Guide

For detailed study of each Jet ORM aspect, see corresponding files in [references/](references/):

### [Models Generation](references/models-generation.md)
- Model generation script from database
- Configuration via Makefile
- Structure of generated files
- Creating Store and dbtx.Executor

**When to read:** During initial project setup or when adding new tables.

### [Query Builder](references/query-builder.md)
- SELECT queries (simple, with JOIN, with FOR UPDATE)
- INSERT queries (simple, with RETURNING)
- UPDATE queries (simple, with conditions)
- DELETE queries

**When to read:** When creating database operations.

### [WHERE Conditions](references/where-conditions.md)
- Simple conditions (EQ, NEQ, GT, LIKE, IN, IS NULL)
- Complex conditions (AND, OR, combinations)
- ORDER BY and LIMIT

**When to read:** When creating queries with filtering and sorting.

### [Data Mapping](references/mapping.md)
- Entity -> Model conversion (toDBEntity)
- Model -> Entity conversion (fromDBEntity)
- Working with nested structures
- Handling nullable fields

**When to read:** When creating mappers for conversion between entity and model.

### [Transactions](references/transactions.md)
- Transaction manager
- Using transactions
- Working with context in transactions

**When to read:** When implementing operations requiring atomicity.

### [Utilities](references/utilities.md)
- UUID conversion for WHERE IN
- ORDER BY and sorting
- LIMIT and OFFSET for pagination
- Useful helper functions

**When to read:** When implementing pagination, bulk operations, or sorting.

## Best Practices

1. **Use type-safe query builder** - Jet provides compile-time type safety
2. **Generate models automatically** - use generation script to sync with database
3. **Separate entity and model** - entity for business logic, model for database
4. **Use mappers** - create toDB* and fromDB* functions for conversion
5. **Work with context** - always pass context to queries
6. **Handle qrm.ErrNoRows** - use to determine record absence
7. **Use FOR UPDATE** - for row locking in transactions
8. **Check SQL** - use stmt.Sql() to view generated SQL

## Resources

- [Jet ORM Documentation](https://github.com/go-jet/jet)
- [Jet v2 Examples](https://github.com/go-jet/jet/tree/master/examples)
- [sqlx Documentation](https://github.com/jmoiron/sqlx)
