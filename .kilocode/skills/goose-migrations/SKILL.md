---
name: goose-migrations
description: Миграции БД через goose (create, up, down, status, versioning). Используй этот скилл, когда нужно управлять миграциями базы данных с помощью goose, создавать новые миграции, применять или откатывать изменения.
license: MIT
metadata:
  category: development
  source:
    repository: project-specific
    path: goose-migrations
---

# Миграции БД через Goose

## Описание
Этот скилл предоставляет руководство по управлению миграциями базы данных с помощью goose - инструмента для миграций на Go. Включает создание миграций, применение (up), откат (down), проверку статуса и версионирование.

## Когда использовать
Используй этот скилл, когда нужно:
- Создавать новые миграции базы данных
- Применять миграции (up)
- Откатывать миграции (down)
- Проверять статус миграций
- Интегрировать миграции с Makefile
- Настраивать goose в docker-compose

## Установка goose

### Установка через Go

```bash
go install github.com/pressly/goose/v3/cmd/goose@v3.26.0
```

### Установка через Makefile

Добавьте в `Makefile`:

```makefile
GOBIN ?= $(PWD)/bin

.PHONY: bin-deps
bin-deps:
	GOBIN=$(GOBIN) go install github.com/pressly/goose/v3/cmd/goose@v3.26.0
```

## Структура миграций

### Директория миграций

```
migrations/
├── 20260105172956_maintenances.sql
├── 20260105175404_maintenance_resources.sql
└── 20260107100500_resources.sql
```

### Формат имени файла

Формат: `YYYYMMDDHHMMSS_<name>.sql`

Пример: `20260105172956_maintenances.sql`

## Создание миграций

### Создание новой миграции

```bash
goose -dir migrations postgres "postgres://user:pass@localhost/db" create "add_users_table" sql
```

### Через Makefile

```makefile
MIGRATIONS_DIR ?= migrations
DB_DRIVER ?= postgres
DB_DSN ?= postgres://user:pass@localhost/db

.PHONY: db-migrate-create
db-migrate-create: name=
db-migrate-create: ## Create new migration
	$(info $(M) creating $(DB_DRIVER) migration...)
	$(GOBIN)/goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) "$(DB_DSN)" create "${name}" sql
```

Использование:

```bash
make db-migrate-create name="add_users_table"
```

## Структура файла миграции

### Базовый шаблон

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE users;
-- +goose StatementEnd
```

### Пример с индексами

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE maintenances (
    id UUID PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    planned_period tstzrange NOT NULL,
    actual_period tstzrange,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ
);

CREATE INDEX maint_planned_period_gist ON maintenances USING GIST (planned_period);
CREATE INDEX maint_actual_period_gist ON maintenances USING GIST (actual_period)
    WHERE actual_period IS NOT NULL;
CREATE INDEX maint_status_idx ON maintenances (status);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS maint_status_idx;
DROP INDEX IF EXISTS maint_actual_period_gist;
DROP INDEX IF EXISTS maint_planned_period_gist;
DROP TABLE maintenances;
-- +goose StatementEnd
```

### Пример с внешними ключами

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE maintenance_resources (
    maintenance_id UUID NOT NULL,
    resource_id UUID NOT NULL,
    resource_type TEXT NOT NULL,
    PRIMARY KEY (maintenance_id, resource_id),
    CONSTRAINT fk_maintenance_resources_maintenance 
        FOREIGN KEY (maintenance_id) 
        REFERENCES maintenances(id) 
        ON DELETE CASCADE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE maintenance_resources;
-- +goose StatementEnd
```

## Применение миграций

### Применение всех миграций (up)

```bash
goose -dir migrations postgres "postgres://user:pass@localhost/db" up
```

### Через Makefile

```makefile
.PHONY: db-up
db-up: ## Apply migrations
	$(info $(M) starting $(DB_DRIVER) migration up...)
	$(GOBIN)/goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) "$(DB_DSN)" up
	$(MAKE) db-status
```

### Применение конкретной миграции

```bash
goose -dir migrations postgres "postgres://user:pass@localhost/db" up 20260105172956
```

## Откат миграций

### Откат одной миграции (down)

```bash
goose -dir migrations postgres "postgres://user:pass@localhost/db" down
```

### Через Makefile

```makefile
.PHONY: db-down
db-down: ## Rollback migrations
	$(info $(M) starting $(DB_DRIVER) migration down...)
	$(GOBIN)/goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) "$(DB_DSN)" down
	$(MAKE) db-status
```

### Откат до конкретной версии

```bash
goose -dir migrations postgres "postgres://user:pass@localhost/db" down 20260105172956
```

## Статус миграций

### Проверка статуса

```bash
goose -dir migrations postgres "postgres://user:pass@localhost/db" status
```

### Через Makefile

```makefile
.PHONY: db-status
db-status: ## Check migrations status
	$(info $(M) check $(DB_DRIVER) migrations status...)
	$(GOBIN)/goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) "$(DB_DSN)" status
```

Пример вывода:

```
20260105172956_maintenances.sql         2025-01-05 17:29:56 +0000 UTC
20260105175404_maintenance_resources.sql 2025-01-05 17:54:04 +0000 UTC
20260107100500_resources.sql            2025-01-07 10:05:00 +0000 UTC
```

## Версионирование миграций

### Таблица goose_db_version

Goose автоматически создаёт таблицу для отслеживания применённых миграций:

```sql
CREATE TABLE goose_db_version (
    id SERIAL PRIMARY KEY,
    version_id BIGINT NOT NULL,
    is_applied BOOLEAN NOT NULL,
    tstamp TIMESTAMP DEFAULT now()
);
```

### Ручная проверка версии

```sql
SELECT * FROM goose_db_version ORDER BY id DESC LIMIT 1;
```

## Интеграция с Docker Compose

### Сервис для применения миграций

```yaml
services:
  postgres:
    image: postgres:18-alpine
    container_name: postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: maintmode
    ports:
      - "5432:5432"
    volumes:
      - db_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 5

  apply-migrations:
    image: kukymbr/goose-docker:3.26.0
    container_name: apply-migrations
    environment:
      GOOSE_DBSTRING: 'postgres://postgres:postgres@postgres:5432/maintmode?sslmode=disable'
      GOOSE_DRIVER: postgres
    volumes:
      - ./migrations:/migrations:ro
    depends_on:
      postgres:
        condition: service_healthy

volumes:
  db_data:
```

### Применение миграций при запуске

```yaml
apply-migrations:
    image: kukymbr/goose-docker:3.26.0
    container_name: apply-migrations
    environment:
      GOOSE_DBSTRING: 'postgres://postgres:postgres@postgres:5432/maintmode?sslmode=disable'
      GOOSE_DRIVER: postgres
    volumes:
      - ./migrations:/migrations:ro
    depends_on:
      postgres:
        condition: service_healthy
```

## Полный Makefile для миграций

```makefile
# Database Configuration
MIGRATIONS_DIR ?= migrations
DB_DRIVER ?= postgres
DB_DSN ?=

ifndef DB_DSN
DB_DSN=$(shell [ -f .env.local ] && cat .env.local | grep ^DB_DSN | awk '{print $$2}' | sed 's/"//g' || echo "")
endif

# db-migrate-create - Create a new migration file
.PHONY: db-migrate-create
db-migrate-create: name=
db-migrate-create: ## Create new migration
	$(info $(M) creating $(DB_DRIVER) migration...)
	$(GOBIN)/goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) "$(DB_DSN)" create "${name}" sql

# db-status - Show migration status
.PHONY: db-status
db-status: ## Check migrations status
	$(info $(M) check $(DB_DRIVER) migrations status...)
	$(GOBIN)/goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) "$(DB_DSN)" status

# db-up - Apply all pending migrations
.PHONY: db-up
db-up: ## Apply migrations
	$(info $(M) starting $(DB_DRIVER) migration up...)
	$(GOBIN)/goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) "$(DB_DSN)" up
	$(MAKE) db-status

# db-down - Rollback the last migration
.PHONY: db-down
db-down: ## Rollback migrations
	$(info $(M) starting $(DB_DRIVER) migration down...)
	$(GOBIN)/goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) "$(DB_DSN)" down
	$(MAKE) db-status
```

## Лучшие практики

1. **Используйте описательные имена миграций** - например, `add_users_table`, `create_maintenances_index`
2. **Всегда пишите и up, и down миграции** - для возможности отката
3. **Используйте транзакции** - goose поддерживает транзакции для атомарных миграций
4. **Не изменяйте существующие миграции** - создайте новую миграцию для изменений
5. **Проверяйте статус перед применением** - используйте `db-status` для проверки текущего состояния
6. **Тестируйте миграции** - проверяйте up/down на тестовой базе данных
7. **Используйте индексы** - добавляйте индексы в up миграции, удаляйте в down
8. **Следите за зависимостями** - учитывайте внешние ключи при создании миграций

## Полезные команды

### Применение миграции с выводом SQL

```bash
goose -dir migrations postgres "postgres://user:pass@localhost/db" up --verbose
```

### Проверка версии goose

```bash
goose -version
```

### Применение миграции с dry-run

```bash
goose -dir migrations postgres "postgres://user:pass@localhost/db" up --dryrun
```

## Ресурсы

- [Goose Documentation](https://github.com/pressly/goose)
- [Goose CLI Reference](https://github.com/pressly/goose#usage)
- [PostgreSQL Migrations Best Practices](https://www.postgresql.org/docs/current/ddl-alter.html)
