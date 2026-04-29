# Генерация моделей Jet

## Скрипт генерации моделей

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

## Запуск генерации через Makefile

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
