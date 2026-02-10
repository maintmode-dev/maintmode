---
name: jet-sqlx
description: Jet ORM + sqlx (query builder, type-safe queries, маппинг). Используй этот скилл, когда нужно работать с Jet ORM v2, sqlx, создавать type-safe queries, настраивать query builder и маппинг данных.
license: MIT
metadata:
  category: development
  source:
    repository: project-specific
    path: jet-sqlx
---

# Jet ORM + sqlx

## Описание
Этот скилл предоставляет руководство по использованию Jet ORM v2 вместе с sqlx для работы с PostgreSQL. Включает type-safe query builder, генерацию моделей, маппинг данных и интеграцию с транзакциями.

## Когда использовать
Используй этот скилл, когда нужно:
- Создавать type-safe SQL запросы с Jet ORM
- Генерировать модели из базы данных
- Настраивать query builder для PostgreSQL
- Работать с маппингом данных (entity <-> model)
- Использовать Jet с sqlx и транзакциями
- Создавать сложные запросы с JOIN, WHERE, ORDER BY

## Установка Jet

### Установка Jet v2

```bash
go get github.com/go-jet/jet/v2/postgres
go get github.com/go-jet/jet/v2/qrm
```

### Зависимости проекта

```go
import (
    "github.com/go-jet/jet/v2/postgres"
    "github.com/go-jet/jet/v2/qrm"
    "github.com/jmoiron/sqlx"
)
```

## Генерация моделей

### Скрипт генерации моделей

Создайте файл `scripts/dbmodels/generate.go`:

```go
package main

import (
    "flag"
    "fmt"
    "os"

    "github.com/go-jet/jet/v2/generator/metadata"
    "github.com/go-jet/jet/v2/generator/postgres"
)

func main() {
    driver := flag.String("driver", "postgres", "Database driver")
    dsn := flag.String("dsn", "", "Database connection string")
    dest := flag.String("dest", "", "Destination directory for generated files")
    flag.Parse()

    if *dsn == "" {
        fmt.Println("DSN is required")
        os.Exit(1)
    }

    if *dest == "" {
        fmt.Println("Destination directory is required")
        os.Exit(1)
    }

    genConfig := postgres.DefaultConfig(
        *driver,
        *dsn,
        *dest,
        metadata.DefaultNamingStrategy,
    )

    err := genConfig.Generate()
    if err != nil {
        fmt.Printf("Error generating models: %v\n", err)
        os.Exit(1)
    }

    fmt.Printf("Models generated successfully in %s\n", *dest)
}
```

### Запуск генерации через Makefile

```makefile
.PHONY: db-models
db-models: ## Generate models
	$(info $(M) generating $(DB_DRIVER) models...)
	@go run ./scripts/dbmodels/generate.go --driver=$(DB_DRIVER) --dsn=$(DB_DSN) --dest="internal/pkg/generated/"
```

### Использование

```bash
make db-models
```

## Структура сгенерированных моделей

```
internal/pkg/generated/
└── postgres/
    └── public/
        ├── model/
        │   ├── maintenances.go
        │   ├── maintenance_resources.go
        │   └── resources.go
        └── table/
            ├── maintenances.go
            ├── maintenance_resources.go
            └── resources.go
```

## Создание Store

### Базовая структура Store

```go
package maintenances

import (
    "github.com/jmoiron/sqlx"
    "github.com/ruko1202/maintmode/internal/utils/dbtx"
)

type Store struct {
    db *dbtx.DB
}

func NewStore(db *sqlx.DB) *Store {
    return &Store{db: dbtx.NewDB(db)}
}
```

### Утилита dbtx.Executor

Создайте файл `internal/utils/dbtx/executor.go`:

```go
package dbtx

import (
    "context"
    "database/sql"

    "github.com/jmoiron/sqlx"
)

type DB struct {
    *sqlx.DB
}

func NewDB(db *sqlx.DB) *DB {
    return &DB{DB: db}
}

func (db *DB) Executor(ctx context.Context) Executor {
    return &executor{db: db.DB, ctx: ctx}
}

type Executor interface {
    QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
    QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
    ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

type executor struct {
    db  *sqlx.DB
    ctx context.Context
}

func (e *executor) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
    return e.db.QueryContext(ctx, query, args...)
}

func (e *executor) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
    return e.db.QueryRowContext(ctx, query, args...)
}

func (e *executor) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
    return e.db.ExecContext(ctx, query, args...)
}
```

## Query Builder

### SELECT запросы

#### Простой SELECT

```go
import (
    "github.com/go-jet/jet/v2/postgres"
    "github.com/ruko1202/maintmode/internal/pkg/generated/postgres/public/model"
    "github.com/ruko1202/maintmode/internal/pkg/generated/postgres/public/table"
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

#### SELECT с FOR UPDATE

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

#### SELECT с JOIN

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

### INSERT запросы

#### Простой INSERT

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

#### INSERT с RETURNING

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

### UPDATE запросы

#### Простой UPDATE

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

#### UPDATE с условием

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

### DELETE запросы

#### Простой DELETE

```go
func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
    stmt := table.Maintenances.DELETE().
        WHERE(table.Maintenances.ID.EQ(postgres.UUID(id)))

    _, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
    return err
}
```

#### DELETE с условием

```go
func (s *Store) DeleteByStatus(ctx context.Context, status string) error {
    stmt := table.Maintenances.DELETE().
        WHERE(table.Maintenances.Status.EQ(postgres.String(status)))

    _, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
    return err
}
```

## WHERE условия

### Простые условия

```go
// EQ
WHERE(table.Maintenances.ID.EQ(postgres.UUID(id)))

// NEQ
WHERE(table.Maintenances.Status.NEQ(postgres.String("deleted")))

// GT, GTE, LT, LTE
WHERE(table.Maintenances.CreatedAt.GTE(postgres.TimestampTz(startTime)))

// LIKE
WHERE(table.Maintenances.Title.LIKE(postgres.String("%maintenance%")))

// IN
WHERE(table.Maintenances.Status.IN(postgres.String("draft"), postgres.String("active")))

// IS NULL
WHERE(table.Maintenances.ActualPeriod.IS_NULL())

// IS NOT NULL
WHERE(table.Maintenances.ActualPeriod.IS_NOT_NULL())
```

### Сложные условия

```go
// AND
WHERE(
    table.Maintenances.Status.EQ(postgres.String("active")).
        AND(table.Maintenances.CreatedAt.GTE(postgres.TimestampTz(startTime))),
)

// OR
WHERE(
    table.Maintenances.Status.EQ(postgres.String("draft")).
        OR(table.Maintenances.Status.EQ(postgres.String("active"))),
)

// Комбинация
WHERE(
    table.Maintenances.Status.EQ(postgres.String("active")).
        AND(
            table.Maintenances.CreatedAt.GTE(postgres.TimestampTz(startTime)).
                OR(table.Maintenances.UpdatedAt.IS_NULL()),
        ),
)
```

## Маппинг данных

### Entity -> Model

```go
package maintenances

import (
    "github.com/google/uuid"
    "github.com/samber/lo"

    "github.com/ruko1202/maintmode/internal/entity"
    "github.com/ruko1202/maintmode/internal/pkg/generated/postgres/public/model"
    "github.com/ruko1202/maintmode/internal/utils/xtime"
)

func toDBMaintenance(m *entity.Maintenance) *model.Maintenances {
    maint := &model.Maintenances{
        ID:                    m.ID,
        Title:                 m.Title,
        Description:           m.Description,
        PlannedPeriod:         xtime.ToPgRange(m.PlannedPeriod),
        Scope:                 string(m.Scope),
        Impact:                string(m.Impact),
        Status:                string(m.Status),
        CanceledReasonCode:    lo.ToPtr(string(m.CancelReason)),
        CanceledReasonComment: lo.ToPtr(m.CancelReasonComment),
        CreatedAt:             m.CreatedAt,
        UpdatedAt:             m.UpdatedAt,
    }

    if m.ActualPeriod != nil {
        actualPeriod := xtime.ToPgRange(lo.FromPtr(m.ActualPeriod))
        maint.ActualPeriod = &actualPeriod
    }

    return maint
}
```

### Model -> Entity

```go
func fromDBMaintenance(m *model.Maintenances) *entity.Maintenance {
    maint := &entity.Maintenance{
        ID:                  m.ID,
        Title:               m.Title,
        Description:         m.Description,
        PlannedPeriod:       xtime.FromPgRange(m.PlannedPeriod),
        Scope:               entity.MaintenanceScope(m.Scope),
        Impact:              entity.MaintenanceImpact(m.Impact),
        Status:              entity.MaintenanceStatus(m.Status),
        CancelReason:        entity.MaintenanceCancelReason(lo.FromPtr(m.CanceledReasonCode)),
        CancelReasonComment: lo.FromPtr(m.CanceledReasonComment),
        CreatedAt:           m.CreatedAt,
        UpdatedAt:           m.UpdatedAt,
    }

    if m.ActualPeriod != nil {
        actualPeriod := xtime.FromPgRange(lo.FromPtr(m.ActualPeriod))
        maint.ActualPeriod = &actualPeriod
    }

    return maint
}
```

## Работа с транзакциями

### Транзакционный менеджер

Создайте файл `internal/utils/dbtx/tx_manager.go`:

```go
package dbtx

import (
    "context"

    "github.com/jmoiron/sqlx"
)

type TxManager struct {
    db *sqlx.DB
}

func NewTxManager(db *sqlx.DB) *TxManager {
    return &TxManager{db: db}
}

func (tm *TxManager) InTx(ctx context.Context, fn func(context.Context) error) error {
    tx, err := tm.db.BeginTxx(ctx, nil)
    if err != nil {
        return err
    }

    defer func() {
        if p := recover(); p != nil {
            _ = tx.Rollback()
            panic(p)
        }
    }()

    txCtx := WithTx(ctx, tx)
    if err := fn(txCtx); err != nil {
        if rbErr := tx.Rollback(); rbErr != nil {
            return rbErr
        }
        return err
    }

    return tx.Commit()
}
```

### Использование транзакций

```go
func (s *Store) CreateWithResources(ctx context.Context, maint *entity.Maintenance, resources []*entity.Resource) error {
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
    if err != nil {
        return err
    }

    for _, resource := range resources {
        dbResource := toDBMaintenanceResource(maint.ID, resource)

        stmt := table.MaintenanceResources.INSERT(
            table.MaintenanceResources.MaintenanceID,
            table.MaintenanceResources.ResourceID,
            table.MaintenanceResources.ResourceType,
        ).MODEL(dbResource)

        _, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
        if err != nil {
            return err
        }
    }

    return nil
}
```

## ORDER BY и LIMIT

### ORDER BY

```go
stmt := table.Maintenances.
    SELECT(table.Maintenances.AllColumns).
    ORDER BY(table.Maintenances.CreatedAt.DESC())

// Множественная сортировка
ORDER BY(
    table.Maintenances.Status.ASC(),
    table.Maintenances.CreatedAt.DESC(),
)
```

### LIMIT и OFFSET

```go
stmt := table.Maintenances.
    SELECT(table.Maintenances.AllColumns).
    ORDER BY(table.Maintenances.CreatedAt.DESC()).
    LIMIT(10).
    OFFSET(0)
```

## Полезные утилиты

### UUID конвертация

```go
func uuidsToPgUUID(resourceIDs []uuid.UUID) []postgres.StringExpression {
    return lo.Map(resourceIDs, func(item uuid.UUID, _ int) postgres.StringExpression {
        return postgres.UUID(item)
    })
}
```

### Использование в WHERE IN

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

    return lo.Map(dbMaintenances, fromDBMaintenance), nil
}
```

## Лучшие практики

1. **Используйте type-safe query builder** - Jet обеспечивает безопасность типов на этапе компиляции
2. **Генерируйте модели автоматически** - используйте скрипт генерации для синхронизации с БД
3. **Разделяйте entity и model** - entity для бизнес-логики, model для базы данных
4. **Используйте мапперы** - создайте функции toDB* и fromDB* для конвертации
5. **Работайте с контекстом** - всегда передавайте context в запросы
6. **Обрабатывайте qrm.ErrNoRows** - используйте для определения отсутствия записи
7. **Используйте FOR UPDATE** - для блокировки записей в транзакциях
8. **Проверяйте SQL** - используйте stmt.Sql() для просмотра сгенерированного SQL

## Ресурсы

- [Jet ORM Documentation](https://github.com/go-jet/jet)
- [Jet v2 Examples](https://github.com/go-jet/jet/tree/master/examples)
- [sqlx Documentation](https://github.com/jmoiron/sqlx)
