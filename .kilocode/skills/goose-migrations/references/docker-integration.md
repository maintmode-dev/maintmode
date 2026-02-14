# Docker Compose Integration

Running goose migrations in Docker Compose environment.

## Service for Applying Migrations

Complete docker-compose configuration with PostgreSQL and migrations:

```yaml
services:
  postgres:
    image: postgres:18-alpine
    container_name: postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: appdb
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
      GOOSE_DBSTRING: 'postgres://postgres:postgres@postgres:5432/appdb?sslmode=disable'
      GOOSE_DRIVER: postgres
    volumes:
      - ./migrations:/migrations:ro
    depends_on:
      postgres:
        condition: service_healthy

volumes:
  db_data:
```

## Migration Service Configuration

### Environment Variables

```yaml
environment:
  GOOSE_DBSTRING: 'postgres://postgres:postgres@postgres:5432/appdb?sslmode=disable'
  GOOSE_DRIVER: postgres
```

**GOOSE_DBSTRING**: PostgreSQL connection string
- Format: `postgres://user:password@host:port/database?sslmode=disable`
- Use service name as host in Docker network (e.g., `postgres`)

**GOOSE_DRIVER**: Database driver (postgres, mysql, sqlite3, etc.)

### Volume Mounting

```yaml
volumes:
  - ./migrations:/migrations:ro
```

Mount local migrations directory as read-only to `/migrations` in container.

### Service Dependencies

```yaml
depends_on:
  postgres:
    condition: service_healthy
```

Wait for PostgreSQL health check to pass before running migrations.

## Apply Migrations on Startup

Minimal configuration for one-time migration application:

```yaml
apply-migrations:
    image: kukymbr/goose-docker:3.26.0
    container_name: apply-migrations
    environment:
      GOOSE_DBSTRING: 'postgres://postgres:postgres@postgres:5432/appdb?sslmode=disable'
      GOOSE_DRIVER: postgres
    volumes:
      - ./migrations:/migrations:ro
    depends_on:
      postgres:
        condition: service_healthy
```

## Usage

### Start all services and apply migrations:
```bash
docker-compose up -d
```

### View migration logs:
```bash
docker logs apply-migrations
```

### Run migrations manually in existing container:
```bash
docker-compose run --rm apply-migrations up
```

### Check migration status:
```bash
docker-compose run --rm apply-migrations status
```

### Rollback last migration:
```bash
docker-compose run --rm apply-migrations down
```

## Best Practices

1. **Use health checks** - Ensure database is ready before migrations
2. **Mount migrations read-only** - Prevent accidental modification
3. **Use service names for networking** - Reference postgres service by name
4. **Set appropriate restart policy** - `unless-stopped` for database
5. **Don't restart migration service** - Remove `restart` policy for one-time runs
6. **Use specific goose version** - Pin to specific tag (e.g., `3.26.0`)

## Integration with Application

Run migrations before starting application:

```yaml
services:
  postgres:
    # ... postgres config ...

  apply-migrations:
    # ... migrations config ...

  app:
    build: .
    container_name: app
    depends_on:
      postgres:
        condition: service_healthy
      apply-migrations:
        condition: service_completed_successfully
    environment:
      DB_DSN: postgres://postgres:postgres@postgres:5432/appdb?sslmode=disable
```

Application waits for both database and migrations to complete.
