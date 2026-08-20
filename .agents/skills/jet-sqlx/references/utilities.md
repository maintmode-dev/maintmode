# Utilities

## UUID conversion

### Function for converting a slice of UUIDs

```go
func uuidsToPgUUID(resourceIDs []uuid.UUID) []postgres.StringExpression {
    return lo.Map(resourceIDs, func(item uuid.UUID, _ int) postgres.StringExpression {
        return postgres.UUID(item)
    })
}
```

### Usage in WHERE IN

```go
func (s *Store) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*entity.Maintenance, error) {
    stmt := table.Maintenances.
        SELECT(table.Maintenances.AllColumns).
        WHERE(table.Maintenances.ID.IN(uuidsToPgUUID(ids)...))

    var dbMaintenances []*model.Maintenances
    err := stmt.QueryContext(ctx, s.db.Executor(ctx), &dbMaintenances)
    if err != nil {
        return nil, err
    }

    return lo.Map(dbMaintenances, func(item *model.Maintenances, _ int) *entity.Maintenance {
        return fromDBMaintenance(item)
    }), nil
}
```

## ORDER BY and sorting

### Simple sorting

```go
stmt := table.Maintenances.
    SELECT(table.Maintenances.AllColumns).
    ORDER_BY(table.Maintenances.CreatedAt.DESC())
```

### Multi-column sorting

```go
ORDER_BY(
    table.Maintenances.Status.ASC(),
    table.Maintenances.CreatedAt.DESC(),
)
```

### Dynamic sorting

```go
func (s *Store) List(ctx context.Context, sortBy string, sortDir string) ([]*entity.Maintenance, error) {
    stmt := table.Maintenances.SELECT(table.Maintenances.AllColumns)

    // Adding ORDER BY dynamically
    switch sortBy {
    case "title":
        if sortDir == "desc" {
            stmt = stmt.ORDER_BY(table.Maintenances.Title.DESC())
        } else {
            stmt = stmt.ORDER_BY(table.Maintenances.Title.ASC())
        }
    case "created_at":
        if sortDir == "desc" {
            stmt = stmt.ORDER_BY(table.Maintenances.CreatedAt.DESC())
        } else {
            stmt = stmt.ORDER_BY(table.Maintenances.CreatedAt.ASC())
        }
    default:
        stmt = stmt.ORDER_BY(table.Maintenances.CreatedAt.DESC())
    }

    var dbMaintenances []*model.Maintenances
    err := stmt.QueryContext(ctx, s.db.Executor(ctx), &dbMaintenances)
    if err != nil {
        return nil, err
    }

    return lo.Map(dbMaintenances, fromDBMaintenance), nil
}
```

## LIMIT and OFFSET for pagination

### Basic pagination

```go
func (s *Store) ListPaginated(ctx context.Context, page, pageSize int) ([]*entity.Maintenance, error) {
    offset := (page - 1) * pageSize

    stmt := table.Maintenances.
        SELECT(table.Maintenances.AllColumns).
        ORDER_BY(table.Maintenances.CreatedAt.DESC()).
        LIMIT(int64(pageSize)).
        OFFSET(int64(offset))

    var dbMaintenances []*model.Maintenances
    err := stmt.QueryContext(ctx, s.db.Executor(ctx), &dbMaintenances)
    if err != nil {
        return nil, err
    }

    return lo.Map(dbMaintenances, fromDBMaintenance), nil
}
```

### Counting the total number of records

```go
import "github.com/go-jet/jet/v2/postgres"

func (s *Store) Count(ctx context.Context) (int64, error) {
    stmt := table.Maintenances.
        SELECT(postgres.COUNT(table.Maintenances.ID).AS("count"))

    var result struct {
        Count int64
    }

    err := stmt.QueryContext(ctx, s.db.Executor(ctx), &result)
    if err != nil {
        return 0, err
    }

    return result.Count, nil
}
```

### Full pagination with metadata

```go
type PaginationResult struct {
    Items      []*entity.Maintenance
    TotalCount int64
    Page       int
    PageSize   int
    TotalPages int
}

func (s *Store) ListWithPagination(ctx context.Context, page, pageSize int) (*PaginationResult, error) {
    // Count the total number of records
    totalCount, err := s.Count(ctx)
    if err != nil {
        return nil, err
    }

    // Fetch the data for the current page
    items, err := s.ListPaginated(ctx, page, pageSize)
    if err != nil {
        return nil, err
    }

    totalPages := int(totalCount) / pageSize
    if int(totalCount)%pageSize != 0 {
        totalPages++
    }

    return &PaginationResult{
        Items:      items,
        TotalCount: totalCount,
        Page:       page,
        PageSize:   pageSize,
        TotalPages: totalPages,
    }, nil
}
```

## Bulk operations

### Bulk INSERT

```go
func (s *Store) BulkCreate(ctx context.Context, maintenances []*entity.Maintenance) error {
    if len(maintenances) == 0 {
        return nil
    }

    models := lo.Map(maintenances, func(item *entity.Maintenance, _ int) *model.Maintenances {
        return toDBMaintenance(item)
    })

    stmt := table.Maintenances.INSERT(
        table.Maintenances.ID,
        table.Maintenances.Title,
        table.Maintenances.Description,
        table.Maintenances.PlannedPeriod,
        table.Maintenances.Status,
        table.Maintenances.Scope,
        table.Maintenances.Impact,
    ).MODELS(models)

    _, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
    return err
}
```

## Debugging SQL

### Getting the generated SQL

```go
stmt := table.Maintenances.
    SELECT(table.Maintenances.AllColumns).
    WHERE(table.Maintenances.ID.EQ(postgres.UUID(id)))

// Print the SQL and its arguments
query, args := stmt.Sql()
fmt.Printf("SQL: %s\n", query)
fmt.Printf("Args: %v\n", args)
```

### Logging queries

```go
func (s *Store) GetWithLogging(ctx context.Context, id uuid.UUID) (*entity.Maintenance, error) {
    stmt := table.Maintenances.
        SELECT(table.Maintenances.AllColumns).
        WHERE(table.Maintenances.ID.EQ(postgres.UUID(id)))

    // Log the SQL
    query, args := stmt.Sql()
    xlog.FromContext(ctx).Debug("Executing query",
        zap.String("sql", query),
        zap.Any("args", args),
    )

    maint := new(model.Maintenances)
    err := stmt.QueryContext(ctx, s.db.Executor(ctx), maint)
    if err != nil {
        return nil, err
    }

    return fromDBMaintenance(maint), nil
}
```
