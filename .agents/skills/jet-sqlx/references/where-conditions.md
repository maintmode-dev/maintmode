# WHERE условия

## Простые условия

```go
// EQ (равно)
WHERE(table.Maintenances.ID.EQ(postgres.UUID(id)))

// NEQ (не равно)
WHERE(table.Maintenances.Status.NEQ(postgres.String("deleted")))

// GT, GTE, LT, LTE (больше, больше-равно, меньше, меньше-равно)
WHERE(table.Maintenances.CreatedAt.GTE(postgres.TimestampTz(startTime)))

// LIKE (поиск по шаблону)
WHERE(table.Maintenances.Title.LIKE(postgres.String("%maintenance%")))

// IN (в списке значений)
WHERE(table.Maintenances.Status.IN(postgres.String("draft"), postgres.String("active")))

// IS NULL (значение NULL)
WHERE(table.Maintenances.ActualPeriod.IS_NULL())

// IS NOT NULL (значение не NULL)
WHERE(table.Maintenances.ActualPeriod.IS_NOT_NULL())
```

## Сложные условия

### AND условие

```go
WHERE(
    table.Maintenances.Status.EQ(postgres.String("active")).
        AND(table.Maintenances.CreatedAt.GTE(postgres.TimestampTz(startTime))),
)
```

### OR условие

```go
WHERE(
    table.Maintenances.Status.EQ(postgres.String("draft")).
        OR(table.Maintenances.Status.EQ(postgres.String("active"))),
)
```

### Комбинация AND и OR

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

### Простая сортировка

```go
stmt := table.Maintenances.
    SELECT(table.Maintenances.AllColumns).
    ORDER_BY(table.Maintenances.CreatedAt.DESC())
```

### Множественная сортировка

```go
ORDER_BY(
    table.Maintenances.Status.ASC(),
    table.Maintenances.CreatedAt.DESC(),
)
```

## LIMIT и OFFSET

```go
stmt := table.Maintenances.
    SELECT(table.Maintenances.AllColumns).
    ORDER_BY(table.Maintenances.CreatedAt.DESC()).
    LIMIT(10).
    OFFSET(0)
```
