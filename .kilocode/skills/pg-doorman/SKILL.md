---
name: pg-doorman
description: PostgreSQL connection pooler configuration, pool modes, and monitoring using pg_doorman from Ozon Tech. Use when configuring connection pooling for PostgreSQL, setting up pool modes (Session/Transaction/Statement), monitoring with Prometheus, integrating pg_doorman with Docker Compose, or optimizing database connections for high-load applications.
license: MIT
metadata:
  category: development
  source:
    repository: project-specific
    path: pg-doorman
---

# PostgreSQL Connection Pooler - pg_doorman

## About pg_doorman

pg_doorman is a connection pooler for PostgreSQL that:
- Manages database connection pool
- Reduces load on PostgreSQL
- Provides metrics for monitoring via Prometheus
- Supports multiple pooling modes (Session, Transaction, Statement)
- Developed and used at Ozon for high-load systems

## Quick Start

### 1. Docker Compose

```yaml
pg_doorman:
  image: ghcr.io/ozontech/pg_doorman:latest
  container_name: pg_doorman
  restart: unless-stopped
  command: ["pg_doorman", "-l", "info", "/etc/pg_doorman/pg_doorman.toml"]
  ports:
    - "6432:6432"  # Client connections
    - "9127:9127"  # Prometheus metrics
  volumes:
    - ./config/pg_doorman.toml:/etc/pg_doorman/pg_doorman.toml:ro
  depends_on:
    postgres:
      condition: service_healthy
```

### 2. Minimal Configuration

```toml
[general]
host = "0.0.0.0"
port = 6432
max_connections = 8192

[prometheus]
host = "0.0.0.0"
port = 9127
enabled = true

[pools.maintmode]
pool_mode = "Transaction"
server_host = "postgres"
server_port = 5432
server_database = "maintmode"

[[pools.maintmode.users]]
username = "postgres"
password = "md53175bce1d3201d16594cebf9d7eb3f9d"
server_username = "postgres"
server_password = "postgres"
pool_size = 40
```

### 3. Application Connection

```go
// DSN through pg_doorman
dsn := "postgres://postgres:postgres@pg_doorman:6432/maintmode?sslmode=disable"

config, err := pgxpool.ParseConfig(dsn)
config.MaxConns = 50
config.MinConns = 20

pool, err := pgxpool.NewWithConfig(ctx, config)
```

### 4. Check Metrics

```bash
curl http://localhost:9127/metrics
```

## Detailed Guide

For detailed study of each pg_doorman aspect, see corresponding files in [references/](references/):

### [Configuration](references/configuration.md)
- Complete pg_doorman.toml configuration
- [general] section parameters
- [prometheus] setup
- [pools] and users configuration
- Timeouts and performance tuning

**When to read:** When setting up production configuration.

### [Pool Modes](references/pool-modes.md)
- Session Mode (persistent connection)
- Transaction Mode (recommended for web applications)
- Statement Mode (for short queries)
- When to use each mode
- Pros and cons of each mode

**When to read:** When choosing optimal mode for your use case.

### [Monitoring](references/monitoring.md)
- Prometheus metrics
- Grafana dashboards
- Health checks
- Troubleshooting

**When to read:** When setting up monitoring and debugging.

## Recommended Mode: Transaction Mode

For most web applications, **Transaction Mode** is recommended:

```toml
[pools.maintmode]
pool_mode = "Transaction"
```

**Why:**
- High efficiency of connection usage
- Good scalability
- Suitable for most scenarios
- Balance between Session and Statement mode

## Generating MD5 Password

```bash
echo -n "postgrespostgres" | md5sum
# Result: 3175bce1d3201d16594cebf9d7eb3f9d
# Add "md5" prefix: md53175bce1d3201d16594cebf9d7eb3f9d
```

## Basic Metrics

| Metric | Description |
|---------|----------|
| `pg_doorman_connections_total` | Total number of connections |
| `pg_doorman_connections_active` | Active connections |
| `pg_doorman_connections_idle` | Idle connections |
| `pg_doorman_pool_size` | Connection pool size |
| `pg_doorman_pool_available` | Available connections in pool |

## Best Practices

1. **Use Transaction mode** for most web applications
2. **Configure pool_size** based on load and number of workers
3. **Enable Prometheus metrics** for performance monitoring
4. **Set up health checks** for automatic recovery
5. **Use md5 hashes** for passwords in configuration
6. **Monitor idle_timeout** to prevent connection leaks
7. **Test configuration** on staging environment before production

## Configuring pool_size

**Recommendations:**
- For small applications: 10-20
- For medium applications: 20-50
- For large applications: 50-100+
- Consider the number of workers/goroutines in your application

## Resources

- [pg_doorman GitHub Repository](https://github.com/ozontech/pg_doorman)
- [pg_doorman Documentation](https://github.com/ozontech/pg_doorman#pg_doorman)
- [PostgreSQL Connection Pooling](https://wiki.postgresql.org/wiki/Connection_pooling)
