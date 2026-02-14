---
name: goose-migrations
description: Database migrations management using goose for creating, applying (up), rolling back (down), and versioning database schema changes. Use when managing database migrations with goose, creating new migration files, applying or rolling back changes, checking migration status, integrating with Makefile and Docker Compose, or setting up migration workflows.
license: MIT
metadata:
  category: development
  source:
    repository: project-specific
    path: goose-migrations
---

# Database Migrations with Goose

## Installing Goose

### Install via Go

```bash
go install github.com/pressly/goose/v3/cmd/goose@v3.26.0
```

### Install via Makefile

Add to `Makefile`:

```makefile
GOBIN ?= $(PWD)/bin

.PHONY: bin-deps
bin-deps:
	GOBIN=$(GOBIN) go install github.com/pressly/goose/v3/cmd/goose@v3.26.0
```

## Migration Structure

### Migrations Directory

```
migrations/
├── 20260105172956_maintenances.sql
├── 20260105175404_maintenance_resources.sql
└── 20260107100500_resources.sql
```

### File Name Format

Format: `YYYYMMDDHHMMSS_<name>.sql`

Example: `20260105172956_maintenances.sql`

## Creating Migrations

### Create New Migration

```bash
goose -dir migrations postgres "postgres://user:pass@localhost/db" create "add_users_table" sql
```

### Via Makefile

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

Usage:

```bash
make db-migrate-create name="add_users_table"
```

## Migration File Structure

### Basic Template

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

### Example with Indexes

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

### Example with Foreign Keys

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

## Applying Migrations

### Apply All Migrations (up)

```bash
goose -dir migrations postgres "postgres://user:pass@localhost/db" up
```

### Via Makefile

```makefile
.PHONY: db-up
db-up: ## Apply migrations
	$(info $(M) starting $(DB_DRIVER) migration up...)
	$(GOBIN)/goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) "$(DB_DSN)" up
	$(MAKE) db-status
```

### Apply Specific Migration

```bash
goose -dir migrations postgres "postgres://user:pass@localhost/db" up 20260105172956
```

## Rolling Back Migrations

### Rollback One Migration (down)

```bash
goose -dir migrations postgres "postgres://user:pass@localhost/db" down
```

### Via Makefile

```makefile
.PHONY: db-down
db-down: ## Rollback migrations
	$(info $(M) starting $(DB_DRIVER) migration down...)
	$(GOBIN)/goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) "$(DB_DSN)" down
	$(MAKE) db-status
```

### Rollback to Specific Version

```bash
goose -dir migrations postgres "postgres://user:pass@localhost/db" down 20260105172956
```

## Migration Status

### Check Status

```bash
goose -dir migrations postgres "postgres://user:pass@localhost/db" status
```

### Via Makefile

```makefile
.PHONY: db-status
db-status: ## Check migrations status
	$(info $(M) check $(DB_DRIVER) migrations status...)
	$(GOBIN)/goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) "$(DB_DSN)" status
```

Example output:

```
20260105172956_maintenances.sql         2025-01-05 17:29:56 +0000 UTC
20260105175404_maintenance_resources.sql 2025-01-05 17:54:04 +0000 UTC
20260107100500_resources.sql            2025-01-07 10:05:00 +0000 UTC
```

## Migration Versioning

### goose_db_version Table

Goose automatically creates a table to track applied migrations:

```sql
CREATE TABLE goose_db_version (
    id SERIAL PRIMARY KEY,
    version_id BIGINT NOT NULL,
    is_applied BOOLEAN NOT NULL,
    tstamp TIMESTAMP DEFAULT now()
);
```

### Manual Version Check

```sql
SELECT * FROM goose_db_version ORDER BY id DESC LIMIT 1;
```

## Docker Integration

For detailed Docker Compose setup, see [Docker Integration](references/docker-integration.md).

**When to read:** When setting up migrations in Docker Compose, running migrations in containers, or integrating migrations with application startup.

## Complete Makefile for Migrations

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

## Best Practices

1. **Use descriptive migration names** - e.g., `add_users_table`, `create_maintenances_index`
2. **Always write both up and down migrations** - for rollback capability
3. **Use transactions** - goose supports transactions for atomic migrations
4. **Don't modify existing migrations** - create a new migration for changes
5. **Check status before applying** - use `db-status` to check current state
6. **Test migrations** - verify up/down on test database
7. **Use indexes** - add indexes in up migrations, remove in down
8. **Track dependencies** - consider foreign keys when creating migrations

## Useful Commands

### Apply Migration with SQL Output

```bash
goose -dir migrations postgres "postgres://user:pass@localhost/db" up --verbose
```

### Check Goose Version

```bash
goose -version
```

### Apply Migration with Dry-Run

```bash
goose -dir migrations postgres "postgres://user:pass@localhost/db" up --dryrun
```

## Resources

- [Goose Documentation](https://github.com/pressly/goose)
- [Goose CLI Reference](https://github.com/pressly/goose#usage)
- [PostgreSQL Migrations Best Practices](https://www.postgresql.org/docs/current/ddl-alter.html)
