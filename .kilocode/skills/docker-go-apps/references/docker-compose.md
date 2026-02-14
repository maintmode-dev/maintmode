# Docker Compose Configuration

Complete docker-compose setup patterns for Go applications with databases and service dependencies.

## Basic Configuration with PostgreSQL

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

volumes:
  db_data:

networks:
  theapp:
    driver: bridge
    name: theapp
```

## Application with Dependencies

```yaml
services:
  pg_doorman:
    image: ghcr.io/ozontech/pg_doorman:latest
    container_name: pg_doorman
    restart: unless-stopped
    command: ["pg_doorman", "-l", "info", "/etc/pg_doorman/pg_doorman.toml"]
    ports:
      - "6432:6432"
      - "9127:9127"
    volumes:
      - ./config/pg_doorman.toml:/etc/pg_doorman/pg_doorman.toml:ro
    depends_on:
      postgres:
        condition: service_healthy

  app:
    build:
      context: .
      dockerfile: ./.build/Dockerfile
    container_name: app
    restart: unless-stopped
    ports:
      - "8000:8000"
      - "8001:8001"
    environment:
      ENVIRONMENT: dev
      DB_DSN: postgres://postgres:postgres@pg_doorman:6432/appdb?sslmode=disable
      DB_DRIVER: postgres
      DB_MAX_OPEN_CONNS: 50
      DB_MAX_IDLE_CONNS: 20
      DB_CONNECTIONS_MAX_LIFETIME: 10m
      DB_CONNECTION_MAX_IDLE_TIME: 5m
    depends_on:
      postgres:
        condition: service_healthy
      pg_doorman:
        condition: service_started
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8001/readiness"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
```

## Health Checks

Health checks allow Docker to monitor container status:

```yaml
healthcheck:
  test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8001/readiness"]
  interval: 30s      # Interval between checks
  timeout: 10s       # Timeout for each check
  retries: 3         # Number of failed attempts before marking as unhealthy
  start_period: 40s  # Time before starting checks after container start
```

## Service Dependencies

Use `depends_on` with `condition` for proper startup order:

```yaml
depends_on:
  postgres:
    condition: service_healthy  # Wait until healthcheck passes
  pg_doorman:
    condition: service_started  # Wait only for container start
```

## Best Practices

1. Always define health checks for critical services
2. Use `condition: service_healthy` for database dependencies
3. Configure appropriate restart policies
4. Use volumes for persistent data
5. Define explicit networks for service communication
6. Use environment variables for configuration
7. Set reasonable health check intervals and timeouts
