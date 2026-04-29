# Query Builder

## SELECT запросы

### Простой SELECT

```go
import (
    "github.com/go-jet/jet/v2/postgres"
    "github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
    "github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) Get(ctx context.Context, maintID uuid.UUID) (*entity.Maintenance, error) {
    stmt := table.Maintenances.
        SELECT(table.Maintenances.AllColumns).
        WHERE(table.Maintenances.ID.EQ(postgres.UUID(maintID)))

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

### SELECT с FOR UPDATE

```go
func (s *Store) GetForUpdate(ctx context.Context, maintID uuid.UUID) (*entity.Maintenance, error) {
    stmt := table.Maintenances.
        SELECT(table.Maintenances.AllColumns).
        WHERE(table.Maintenances.ID.EQ(postgres.UUID(maintID))).
        FOR(postgres.UPDATE())

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

### SELECT с JOIN

```go
func (s *Store) GetWithResources(ctx context.Context, maintID uuid.UUID) (*entity.Maintenance, error) {
    stmt := table.Maintenances.
        SELECT(
            table.Maintenances.AllColumns,
            table.MaintenanceResources.AllColumns,
        ).
        FROM(table.Maintenances.
            INNER_JOIN(table.MaintenanceResources,
                table.Maintenances.ID.EQ(table.MaintenanceResources.MaintenanceID),
            ),
        ).
        WHERE(table.Maintenances.ID.EQ(postgres.UUID(maintID)))

    var result struct {
        Maintenance *model.Maintenances
        Resource    *model.MaintenanceResources
    }

    err := stmt.QueryContext(ctx, s.db.Executor(ctx), &result)
    if err != nil {
        return nil, err
    }

    return fromDBMaintenance(result.Maintenance), nil
}
```

## INSERT запросы

### Простой INSERT

```go
func (s *Store) Create(ctx context.Context, maint *entity.Maintenance) error {
    dbMaint := toDBMaintenance(maint)

    stmt := table.Maintenances.INSERT(
        table.Maintenances.ID,
        table.Maintenances.Title,
        table.Maintenances.Description,
        table.Maintenances.PlannedPeriod,
        table.Maintenances.Status,
        table.Maintenances.Scope,
        table.Maintenances.Impact,
    ).MODEL(dbMaint)

    _, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
    return err
}
```

### INSERT с RETURNING

```go
func (s *Store) CreateWithReturning(ctx context.Context, maint *entity.Maintenance) (*entity.Maintenance, error) {
    dbMaint := toDBMaintenance(maint)

    stmt := table.Maintenances.INSERT(
        table.Maintenances.ID,
        table.Maintenances.Title,
        table.Maintenances.Description,
        table.Maintenances.PlannedPeriod,
        table.Maintenances.Status,
        table.Maintenances.Scope,
        table.Maintenances.Impact,
    ).MODEL(dbMaint).RETURNING(table.Maintenances.AllColumns)

    result := new(model.Maintenances)
    err := stmt.QueryContext(ctx, s.db.Executor(ctx), result)
    if err != nil {
        return nil, err
    }

    return fromDBMaintenance(result), nil
}
```

## UPDATE запросы

### Простой UPDATE

```go
func (s *Store) Update(ctx context.Context, maint *entity.Maintenance) error {
    dbMaint := toDBMaintenance(maint)

    stmt := table.Maintenances.
        UPDATE(
            table.Maintenances.Title,
            table.Maintenances.Description,
            table.Maintenances.Status,
            table.Maintenances.UpdatedAt,
        ).
        MODEL(dbMaint).
        WHERE(table.Maintenances.ID.EQ(postgres.UUID(maint.ID)))

    _, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
    return err
}
```

### UPDATE с условием

```go
func (s *Store) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
    stmt := table.Maintenances.
        UPDATE(table.Maintenances.Status).
        SET(table.Maintenances.Status.SET(status)).
        WHERE(table.Maintenances.ID.EQ(postgres.UUID(id)))

    _, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
    return err
}
```

## DELETE запросы

### Простой DELETE

```go
func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
    stmt := table.Maintenances.DELETE().
        WHERE(table.Maintenances.ID.EQ(postgres.UUID(id)))

    _, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
    return err
}
```

### DELETE с условием

```go
func (s *Store) DeleteByStatus(ctx context.Context, status string) error {
    stmt := table.Maintenances.DELETE().
        WHERE(table.Maintenances.Status.EQ(postgres.String(status)))

    _, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
    return err
}
```
