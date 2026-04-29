# Работа с транзакциями

## Транзакционный менеджер

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

## Использование транзакций

### Пример с несколькими операциями

```go
func (s *Store) CreateWithResources(ctx context.Context, maint *entity.Maintenance, resources []*entity.Resource) error {
    dbMaint := toDBMaintenance(maint)

    // Вставка основной записи
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

    // Вставка связанных ресурсов
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

### Использование через TxManager

```go
func (s *Service) CreateMaintenanceWithResources(ctx context.Context, req *CreateRequest) error {
    return s.txManager.InTx(ctx, func(txCtx context.Context) error {
        // Создание maintenance
        maint, err := s.store.Create(txCtx, req.Maintenance)
        if err != nil {
            return err
        }

        // Создание связанных ресурсов
        for _, resource := range req.Resources {
            if err := s.resourceStore.Link(txCtx, maint.ID, resource.ID); err != nil {
                return err
            }
        }

        return nil
    })
}
```

## Работа с контекстом в транзакциях

### Передача транзакции через контекст

```go
type txKey struct{}

func WithTx(ctx context.Context, tx *sqlx.Tx) context.Context {
    return context.WithValue(ctx, txKey{}, tx)
}

func TxFromContext(ctx context.Context) (*sqlx.Tx, bool) {
    tx, ok := ctx.Value(txKey{}).(*sqlx.Tx)
    return tx, ok
}
```

### Executor с поддержкой транзакций

```go
func (db *DB) Executor(ctx context.Context) Executor {
    if tx, ok := TxFromContext(ctx); ok {
        return &executor{db: nil, tx: tx, ctx: ctx}
    }
    return &executor{db: db.DB, tx: nil, ctx: ctx}
}

type executor struct {
    db  *sqlx.DB
    tx  *sqlx.Tx
    ctx context.Context
}

func (e *executor) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
    if e.tx != nil {
        return e.tx.ExecContext(ctx, query, args...)
    }
    return e.db.ExecContext(ctx, query, args...)
}
```

## Лучшие практики

1. **Используйте короткие транзакции** - держите транзакции максимально короткими
2. **Обрабатывайте ошибки** - всегда проверяйте ошибки и делайте rollback
3. **Используйте defer для rollback** - гарантирует rollback даже при panic
4. **Передавайте контекст** - используйте context для передачи транзакции между слоями
5. **Избегайте вложенных транзакций** - если транзакция уже существует, используйте её
