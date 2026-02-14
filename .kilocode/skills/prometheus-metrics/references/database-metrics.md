# Database Metrics for PostgreSQL

Complete guide for instrumenting PostgreSQL queries with Prometheus metrics in Go applications using sqlx.

## Table of Contents
- [Query Duration Tracking](#query-duration-tracking)
- [Connection Pool Metrics](#connection-pool-metrics)
- [Query Error Tracking](#query-error-tracking)
- [Custom Database Wrapper](#custom-database-wrapper)
- [Integration with Jet/SQLX](#integration-with-jetsqlx)

## Query Duration Tracking

Track PostgreSQL query performance:

```go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    dbQueryDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "db_query_duration_seconds",
            Help:    "Database query duration in seconds",
            Buckets: []float64{0.0001, 0.001, 0.01, 0.1, 0.5, 1.0}, // 0.1ms to 1s
        },
        []string{"operation", "table"},
    )

    dbQueriesTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "db_queries_total",
            Help: "Total number of database queries",
        },
        []string{"operation", "table"},
    )

    dbQueryErrors = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "db_query_errors_total",
            Help: "Total number of database query errors",
        },
        []string{"operation", "table"},
    )
)
```

## Connection Pool Metrics

Monitor sqlx connection pool health:

```go
package metrics

import (
    "context"
    "time"

    "github.com/jmoiron/sqlx"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    dbConnectionsOpen = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "db_connections_open",
            Help: "Number of open database connections",
        },
    )

    dbConnectionsInUse = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "db_connections_in_use",
            Help: "Number of database connections currently in use",
        },
    )

    dbConnectionsIdle = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "db_connections_idle",
            Help: "Number of idle database connections",
        },
    )

    dbConnectionWaitDuration = promauto.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "db_connection_wait_duration_seconds",
            Help:    "Time spent waiting for database connection",
            Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1.0},
        },
    )
)

// Start collecting pool metrics
func StartPoolMetricsCollector(db *sqlx.DB, interval time.Duration) {
    go func() {
        ticker := time.NewTicker(interval)
        defer ticker.Stop()

        for range ticker.C {
            stats := db.Stats()

            dbConnectionsOpen.Set(float64(stats.OpenConnections))
            dbConnectionsInUse.Set(float64(stats.InUse))
            dbConnectionsIdle.Set(float64(stats.Idle))

            // Wait duration requires custom tracking (see below)
        }
    }()
}
```

## Query Error Tracking

Track database errors by type:

```go
package metrics

import (
    "database/sql"
    "errors"

    "github.com/lib/pq"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    dbErrors = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "db_errors_total",
            Help: "Total number of database errors",
        },
        []string{"error_type"},
    )
)

func RecordDatabaseError(err error) {
    if err == nil {
        return
    }

    errorType := classifyError(err)
    dbErrors.WithLabelValues(errorType).Inc()
}

func classifyError(err error) string {
    if errors.Is(err, sql.ErrNoRows) {
        return "no_rows"
    }

    if errors.Is(err, sql.ErrConnDone) {
        return "connection_closed"
    }

    // PostgreSQL specific errors
    if pqErr, ok := err.(*pq.Error); ok {
        switch pqErr.Code {
        case "23505": // unique_violation
            return "unique_violation"
        case "23503": // foreign_key_violation
            return "foreign_key_violation"
        case "23502": // not_null_violation
            return "not_null_violation"
        case "23514": // check_violation
            return "check_violation"
        default:
            return "postgres_error"
        }
    }

    return "unknown"
}
```

## Custom Database Wrapper

Wrap database operations for automatic metrics:

```go
package dbtx

import (
    "context"
    "database/sql"
    "time"

    "github.com/jmoiron/sqlx"
    "github.com/prometheus/client_golang/prometheus"
)

// Executor interface from existing codebase
type Executor interface {
    QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
    QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
    ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// MetricsExecutor wraps Executor with Prometheus metrics
type MetricsExecutor struct {
    Executor
    operation string
    table     string
}

func NewMetricsExecutor(ex Executor, operation, table string) *MetricsExecutor {
    return &MetricsExecutor{
        Executor:  ex,
        operation: operation,
        table:     table,
    }
}

func (m *MetricsExecutor) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
    start := time.Now()
    rows, err := m.Executor.QueryContext(ctx, query, args...)
    duration := time.Since(start).Seconds()

    dbQueriesTotal.WithLabelValues(m.operation, m.table).Inc()
    dbQueryDuration.WithLabelValues(m.operation, m.table).Observe(duration)

    if err != nil {
        dbQueryErrors.WithLabelValues(m.operation, m.table).Inc()
        RecordDatabaseError(err)
    }

    return rows, err
}

func (m *MetricsExecutor) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
    start := time.Now()
    row := m.Executor.QueryRowContext(ctx, query, args...)
    duration := time.Since(start).Seconds()

    dbQueriesTotal.WithLabelValues(m.operation, m.table).Inc()
    dbQueryDuration.WithLabelValues(m.operation, m.table).Observe(duration)

    // Note: QueryRow errors are deferred until Scan()
    // Consider wrapping *sql.Row to track errors

    return row
}

func (m *MetricsExecutor) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
    start := time.Now()
    result, err := m.Executor.ExecContext(ctx, query, args...)
    duration := time.Since(start).Seconds()

    dbQueriesTotal.WithLabelValues(m.operation, m.table).Inc()
    dbQueryDuration.WithLabelValues(m.operation, m.table).Observe(duration)

    if err != nil {
        dbQueryErrors.WithLabelValues(m.operation, m.table).Inc()
        RecordDatabaseError(err)
    }

    return result, err
}
```

## Integration with Jet/SQLX

Instrument existing MaintMode storage layer:

```go
package maintenances

import (
    "context"

    "github.com/go-jet/jet/v2/postgres"
    "github.com/jmoiron/sqlx"

    "github.com/ruko1202/maintmode/internal/utils/dbtx"
    "github.com/ruko1202/maintmode/internal/metrics"
)

type Store struct {
    db *dbtx.DB
}

func NewStore(db *sqlx.DB) *Store {
    return &Store{db: dbtx.NewDB(db)}
}

// Create with metrics
func (s *Store) Create(ctx context.Context, maint *entity.Maintenance) error {
    executor := metrics.NewMetricsExecutor(
        s.db.Executor(ctx),
        "insert",
        "maintenances",
    )

    stmt := postgres.Maintenances.INSERT(
        postgres.Maintenances.AllColumns,
    ).MODEL(maint)

    _, err := stmt.ExecContext(ctx, executor)
    return err
}

// Get with metrics
func (s *Store) Get(ctx context.Context, id uuid.UUID) (*entity.Maintenance, error) {
    executor := metrics.NewMetricsExecutor(
        s.db.Executor(ctx),
        "select",
        "maintenances",
    )

    stmt := postgres.Maintenances.
        SELECT(postgres.Maintenances.AllColumns).
        WHERE(postgres.Maintenances.ID.EQ(postgres.UUID(id)))

    var maint entity.Maintenance
    err := stmt.QueryContext(ctx, executor).Scan(&maint)
    return &maint, err
}

// Update with metrics
func (s *Store) Update(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
    executor := metrics.NewMetricsExecutor(
        s.db.Executor(ctx),
        "update",
        "maintenances",
    )

    stmt := postgres.Maintenances.UPDATE(
        // update columns
    ).WHERE(postgres.Maintenances.ID.EQ(postgres.UUID(id)))

    _, err := stmt.ExecContext(ctx, executor)
    return err
}
```

## Custom Collector for Advanced Stats

Collect custom database statistics:

```go
package metrics

import (
    "github.com/jmoiron/sqlx"
    "github.com/prometheus/client_golang/prometheus"
)

type dbStatsCollector struct {
    db *sqlx.DB

    openConnections      *prometheus.Desc
    inUseConnections     *prometheus.Desc
    idleConnections      *prometheus.Desc
    waitCount            *prometheus.Desc
    waitDuration         *prometheus.Desc
    maxIdleClosed        *prometheus.Desc
    maxIdleTimeClosed    *prometheus.Desc
    maxLifetimeClosed    *prometheus.Desc
}

func NewDBStatsCollector(db *sqlx.DB) prometheus.Collector {
    return &dbStatsCollector{
        db: db,
        openConnections: prometheus.NewDesc(
            "db_connections_open",
            "Number of open database connections",
            nil, nil,
        ),
        inUseConnections: prometheus.NewDesc(
            "db_connections_in_use",
            "Number of connections currently in use",
            nil, nil,
        ),
        idleConnections: prometheus.NewDesc(
            "db_connections_idle",
            "Number of idle connections",
            nil, nil,
        ),
        waitCount: prometheus.NewDesc(
            "db_connection_wait_count_total",
            "Total number of connections waited for",
            nil, nil,
        ),
        waitDuration: prometheus.NewDesc(
            "db_connection_wait_duration_seconds_total",
            "Total time waited for connections",
            nil, nil,
        ),
        maxIdleClosed: prometheus.NewDesc(
            "db_connections_max_idle_closed_total",
            "Total connections closed due to max idle",
            nil, nil,
        ),
        maxIdleTimeClosed: prometheus.NewDesc(
            "db_connections_max_idle_time_closed_total",
            "Total connections closed due to max idle time",
            nil, nil,
        ),
        maxLifetimeClosed: prometheus.NewDesc(
            "db_connections_max_lifetime_closed_total",
            "Total connections closed due to max lifetime",
            nil, nil,
        ),
    }
}

func (c *dbStatsCollector) Describe(ch chan<- *prometheus.Desc) {
    ch <- c.openConnections
    ch <- c.inUseConnections
    ch <- c.idleConnections
    ch <- c.waitCount
    ch <- c.waitDuration
    ch <- c.maxIdleClosed
    ch <- c.maxIdleTimeClosed
    ch <- c.maxLifetimeClosed
}

func (c *dbStatsCollector) Collect(ch chan<- prometheus.Metric) {
    stats := c.db.Stats()

    ch <- prometheus.MustNewConstMetric(
        c.openConnections,
        prometheus.GaugeValue,
        float64(stats.OpenConnections),
    )
    ch <- prometheus.MustNewConstMetric(
        c.inUseConnections,
        prometheus.GaugeValue,
        float64(stats.InUse),
    )
    ch <- prometheus.MustNewConstMetric(
        c.idleConnections,
        prometheus.GaugeValue,
        float64(stats.Idle),
    )
    ch <- prometheus.MustNewConstMetric(
        c.waitCount,
        prometheus.CounterValue,
        float64(stats.WaitCount),
    )
    ch <- prometheus.MustNewConstMetric(
        c.waitDuration,
        prometheus.CounterValue,
        stats.WaitDuration.Seconds(),
    )
    ch <- prometheus.MustNewConstMetric(
        c.maxIdleClosed,
        prometheus.CounterValue,
        float64(stats.MaxIdleClosed),
    )
    ch <- prometheus.MustNewConstMetric(
        c.maxIdleTimeClosed,
        prometheus.CounterValue,
        float64(stats.MaxIdleTimeClosed),
    )
    ch <- prometheus.MustNewConstMetric(
        c.maxLifetimeClosed,
        prometheus.CounterValue,
        float64(stats.MaxLifetimeClosed),
    )
}

// Register the collector
func RegisterDBStatsCollector(db *sqlx.DB) {
    prometheus.MustRegister(NewDBStatsCollector(db))
}
```

## Transaction Metrics

Track transaction duration and outcomes:

```go
package metrics

var (
    dbTransactionDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "db_transaction_duration_seconds",
            Help:    "Database transaction duration",
            Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1.0, 5.0},
        },
        []string{"outcome"}, // "commit", "rollback"
    )
)

// Wrap transaction with metrics
func (s *Store) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
    start := time.Now()

    err := s.db.WithTransaction(ctx, fn)

    duration := time.Since(start).Seconds()
    outcome := "commit"
    if err != nil {
        outcome = "rollback"
    }

    dbTransactionDuration.WithLabelValues(outcome).Observe(duration)

    return err
}
```

## Best Practices

1. **Use operation + table labels** for query metrics, not full SQL
2. **Monitor connection pool** to detect leaks and starvation
3. **Track errors by type** to identify systematic issues
4. **Use histograms** for query duration (not summaries)
5. **Choose appropriate buckets** - database queries are typically < 100ms
6. **Register pool collector** for automatic stats collection
7. **Coordinate with logging** - metrics for trends, logs for debugging
8. **Test metrics wrappers** to ensure they don't break existing code
9. **Avoid high-cardinality labels** - don't use query parameters or IDs
10. **Use custom collectors** for aggregated statistics
