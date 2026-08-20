# WHERE conditions

## Simple conditions

```go
// EQ (equal to)
WHERE(table.Maintenances.ID.EQ(postgres.UUID(id)))

// NEQ (not equal to)
WHERE(table.Maintenances.Status.NEQ(postgres.String("deleted")))

// GT, GTE, LT, LTE (greater than, greater than or equal, less than, less than or equal)
WHERE(table.Maintenances.CreatedAt.GTE(postgres.TimestampTz(startTime)))

// LIKE (pattern matching)
WHERE(table.Maintenances.Title.LIKE(postgres.String("%maintenance%")))

// IN (within a list of values)
WHERE(table.Maintenances.Status.IN(postgres.String("draft"), postgres.String("active")))

// IS NULL (the value is NULL)
WHERE(table.Maintenances.ActualPeriod.IS_NULL())

// IS NOT NULL (the value is not NULL)
WHERE(table.Maintenances.ActualPeriod.IS_NOT_NULL())
```

## Complex conditions

### AND condition

```go
WHERE(
    table.Maintenances.Status.EQ(postgres.String("active")).
        AND(table.Maintenances.CreatedAt.GTE(postgres.TimestampTz(startTime))),
)
```

### OR condition

```go
WHERE(
    table.Maintenances.Status.EQ(postgres.String("draft")).
        OR(table.Maintenances.Status.EQ(postgres.String("active"))),
)
```

### Combining AND and OR

```go
WHERE(
    table.Maintenances.Status.EQ(postgres.String("active")).
        AND(
            table.Maintenances.CreatedAt.GTE(postgres.TimestampTz(startTime)).
                OR(table.Maintenances.UpdatedAt.IS_NULL()),
        ),
)
```

## ORDER BY

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

## LIMIT and OFFSET

```go
stmt := table.Maintenances.
    SELECT(table.Maintenances.AllColumns).
    ORDER_BY(table.Maintenances.CreatedAt.DESC()).
    LIMIT(10).
    OFFSET(0)
```
