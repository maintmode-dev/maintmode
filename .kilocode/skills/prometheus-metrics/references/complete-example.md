# Complete Prometheus Metrics Example

Full working implementation integrating HTTP, database, and business metrics for MaintMode application.

## Project Structure

```
internal/
├── metrics/
│   ├── http.go           # HTTP metrics
│   ├── database.go       # Database metrics
│   ├── business.go       # Business metrics
│   └── registry.go       # Metrics registration
├── server/
│   └── api_server.go     # Echo server setup
├── storages/
│   └── maintenances/
│       └── store.go      # Storage layer with metrics
└── services/
    └── maint/
        └── service.go    # Service layer with metrics
```

## 1. Metrics Package

### metrics/http.go

```go
package metrics

import (
    "strconv"
    "time"

    "github.com/labstack/echo/v4"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    httpRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "maintmode",
            Subsystem: "http",
            Name:      "requests_total",
            Help:      "Total number of HTTP requests",
        },
        []string{"method", "path", "status"},
    )

    httpRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Namespace: "maintmode",
            Subsystem: "http",
            Name:      "request_duration_seconds",
            Help:      "HTTP request duration in seconds",
            Buckets:   []float64{0.001, 0.01, 0.1, 0.5, 1.0, 5.0},
        },
        []string{"method", "path"},
    )

    httpRequestSize = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Namespace: "maintmode",
            Subsystem: "http",
            Name:      "request_size_bytes",
            Help:      "HTTP request size in bytes",
            Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
        },
        []string{"method", "path"},
    )

    httpResponseSize = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Namespace: "maintmode",
            Subsystem: "http",
            Name:      "response_size_bytes",
            Help:      "HTTP response size in bytes",
            Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
        },
        []string{"method", "path"},
    )

    httpRequestsActive = promauto.NewGauge(
        prometheus.GaugeOpts{
            Namespace: "maintmode",
            Subsystem: "http",
            Name:      "requests_active",
            Help:      "Number of active HTTP requests",
        },
    )
)

// PrometheusMiddleware returns Echo middleware for Prometheus metrics
func PrometheusMiddleware() echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            start := time.Now()

            // Track active requests
            httpRequestsActive.Inc()
            defer httpRequestsActive.Dec()

            // Track request size
            reqSize := int(c.Request().ContentLength)
            if reqSize < 0 {
                reqSize = 0
            }

            // Execute handler
            err := next(c)

            // Record metrics
            duration := time.Since(start).Seconds()
            status := strconv.Itoa(c.Response().Status)
            method := c.Request().Method
            path := c.Path() // Use route path, not actual URL

            httpRequestsTotal.WithLabelValues(method, path, status).Inc()
            httpRequestDuration.WithLabelValues(method, path).Observe(duration)
            httpRequestSize.WithLabelValues(method, path).Observe(float64(reqSize))
            httpResponseSize.WithLabelValues(method, path).Observe(float64(c.Response().Size))

            return err
        }
    }
}
```

### metrics/database.go

```go
package metrics

import (
    "context"
    "database/sql"
    "time"

    "github.com/jmoiron/sqlx"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"

    "github.com/ruko1202/maintmode/internal/utils/dbtx"
)

var (
    dbQueryDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Namespace: "maintmode",
            Subsystem: "db",
            Name:      "query_duration_seconds",
            Help:      "Database query duration in seconds",
            Buckets:   []float64{0.0001, 0.001, 0.01, 0.1, 0.5, 1.0},
        },
        []string{"operation", "table"},
    )

    dbQueriesTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "maintmode",
            Subsystem: "db",
            Name:      "queries_total",
            Help:      "Total number of database queries",
        },
        []string{"operation", "table"},
    )

    dbQueryErrors = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "maintmode",
            Subsystem: "db",
            Name:      "query_errors_total",
            Help:      "Total number of database query errors",
        },
        []string{"operation", "table"},
    )

    dbConnectionsOpen = promauto.NewGauge(
        prometheus.GaugeOpts{
            Namespace: "maintmode",
            Subsystem: "db",
            Name:      "connections_open",
            Help:      "Number of open database connections",
        },
    )

    dbConnectionsInUse = promauto.NewGauge(
        prometheus.GaugeOpts{
            Namespace: "maintmode",
            Subsystem: "db",
            Name:      "connections_in_use",
            Help:      "Number of database connections in use",
        },
    )

    dbConnectionsIdle = promauto.NewGauge(
        prometheus.GaugeOpts{
            Namespace: "maintmode",
            Subsystem: "db",
            Name:      "connections_idle",
            Help:      "Number of idle database connections",
        },
    )
)

// MetricsExecutor wraps dbtx.Executor with Prometheus metrics
type MetricsExecutor struct {
    dbtx.Executor
    operation string
    table     string
}

// NewMetricsExecutor creates a new metrics-enabled executor
func NewMetricsExecutor(ex dbtx.Executor, operation, table string) *MetricsExecutor {
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
    }

    return rows, err
}

func (m *MetricsExecutor) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
    start := time.Now()
    row := m.Executor.QueryRowContext(ctx, query, args...)
    duration := time.Since(start).Seconds()

    dbQueriesTotal.WithLabelValues(m.operation, m.table).Inc()
    dbQueryDuration.WithLabelValues(m.operation, m.table).Observe(duration)

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
    }

    return result, err
}

// StartPoolMetricsCollector starts collecting database pool metrics
func StartPoolMetricsCollector(db *sqlx.DB, interval time.Duration) {
    go func() {
        ticker := time.NewTicker(interval)
        defer ticker.Stop()

        for range ticker.C {
            stats := db.Stats()

            dbConnectionsOpen.Set(float64(stats.OpenConnections))
            dbConnectionsInUse.Set(float64(stats.InUse))
            dbConnectionsIdle.Set(float64(stats.Idle))
        }
    }()
}
```

### metrics/business.go

```go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    maintenancesCreated = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "maintmode",
            Subsystem: "maintenance",
            Name:      "created_total",
            Help:      "Total number of maintenances created",
        },
        []string{"status"},
    )

    maintenanceTransitions = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "maintmode",
            Subsystem: "maintenance",
            Name:      "transitions_total",
            Help:      "Total maintenance state transitions",
        },
        []string{"from_status", "to_status"},
    )

    maintenancesCurrent = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Namespace: "maintmode",
            Subsystem: "maintenance",
            Name:      "current",
            Help:      "Current number of maintenances by status",
        },
        []string{"status"},
    )

    maintenanceDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Namespace: "maintmode",
            Subsystem: "maintenance",
            Name:      "duration_seconds",
            Help:      "Maintenance duration in seconds",
            Buckets:   []float64{60, 300, 900, 1800, 3600, 7200, 14400},
        },
        []string{"status"},
    )

    conflictsDetected = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "maintmode",
            Subsystem: "maintenance",
            Name:      "conflicts_detected_total",
            Help:      "Total conflicts detected",
        },
        []string{"conflict_type"},
    )

    businessOperations = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "maintmode",
            Subsystem: "business",
            Name:      "operations_total",
            Help:      "Total business operations",
        },
        []string{"operation", "outcome"},
    )
)

// RecordMaintenanceCreated tracks maintenance creation
func RecordMaintenanceCreated(status string) {
    maintenancesCreated.WithLabelValues(status).Inc()
    maintenancesCurrent.WithLabelValues(status).Inc()
}

// RecordMaintenanceTransition tracks status transitions
func RecordMaintenanceTransition(fromStatus, toStatus string) {
    maintenanceTransitions.WithLabelValues(fromStatus, toStatus).Inc()
    maintenancesCurrent.WithLabelValues(fromStatus).Dec()
    maintenancesCurrent.WithLabelValues(toStatus).Inc()
}

// RecordMaintenanceCompleted tracks completed maintenance
func RecordMaintenanceCompleted(durationSeconds float64) {
    maintenanceDuration.WithLabelValues("completed").Observe(durationSeconds)
}

// RecordConflictDetected tracks conflict detection
func RecordConflictDetected(conflictType string) {
    conflictsDetected.WithLabelValues(conflictType).Inc()
}

// RecordBusinessOperation tracks business operation outcome
func RecordBusinessOperation(operation, outcome string) {
    businessOperations.WithLabelValues(operation, outcome).Inc()
}
```

## 2. Server Integration

### server/api_server.go

```go
package server

import (
    "context"
    "time"

    "github.com/labstack/echo/v4"
    "github.com/prometheus/client_golang/prometheus/promhttp"

    "github.com/ruko1202/maintmode/internal/config"
    "github.com/ruko1202/maintmode/internal/config/middlewares"
    "github.com/ruko1202/maintmode/internal/metrics"
)

func NewAPIServer(cfg config.HTTPServer, db *sqlx.DB) *server {
    e := echo.New()

    // Register base middlewares
    for _, mw := range middlewares.BaseMiddlewares() {
        e.Use(mw)
    }

    // Add Prometheus middleware (before logging)
    e.Use(metrics.PrometheusMiddleware())

    // Expose /metrics endpoint
    e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

    // Start database pool metrics collector
    metrics.StartPoolMetricsCollector(db, 15*time.Second)

    return &server{
        cfg: cfg,
        e:   e,
    }
}
```

## 3. Storage Layer Integration

### storages/maintenances/store.go

```go
package maintenances

import (
    "context"

    "github.com/google/uuid"
    "github.com/jmoiron/sqlx"

    "github.com/ruko1202/maintmode/internal/entity"
    "github.com/ruko1202/maintmode/internal/metrics"
    "github.com/ruko1202/maintmode/internal/utils/dbtx"
)

type Store struct {
    db *dbtx.DB
}

func NewStore(db *sqlx.DB) *Store {
    return &Store{db: dbtx.NewDB(db)}
}

func (s *Store) Create(ctx context.Context, maint *entity.Maintenance) error {
    executor := metrics.NewMetricsExecutor(
        s.db.Executor(ctx),
        "insert",
        "maintenances",
    )

    // Execute insert query with metrics
    query := `
        INSERT INTO maintenances (id, title, status, start_time, end_time, created_at)
        VALUES ($1, $2, $3, $4, $5, $6)
    `

    _, err := executor.ExecContext(ctx, query,
        maint.ID, maint.Title, maint.Status, maint.StartTime, maint.EndTime, maint.CreatedAt,
    )

    return err
}

func (s *Store) Get(ctx context.Context, id uuid.UUID) (*entity.Maintenance, error) {
    executor := metrics.NewMetricsExecutor(
        s.db.Executor(ctx),
        "select",
        "maintenances",
    )

    var maint entity.Maintenance
    query := `SELECT * FROM maintenances WHERE id = $1`

    err := executor.QueryRowContext(ctx, query, id).Scan(
        &maint.ID, &maint.Title, &maint.Status, &maint.StartTime, &maint.EndTime, &maint.CreatedAt,
    )

    if err != nil {
        return nil, err
    }

    return &maint, nil
}

func (s *Store) UpdateStatus(ctx context.Context, id uuid.UUID, status entity.MaintenanceStatus) error {
    executor := metrics.NewMetricsExecutor(
        s.db.Executor(ctx),
        "update",
        "maintenances",
    )

    query := `UPDATE maintenances SET status = $1 WHERE id = $2`
    _, err := executor.ExecContext(ctx, query, status, id)

    return err
}
```

## 4. Service Layer Integration

### services/maint/service.go

```go
package maint

import (
    "context"
    "time"

    "github.com/google/uuid"

    "github.com/ruko1202/maintmode/internal/entity"
    "github.com/ruko1202/maintmode/internal/metrics"
    "github.com/ruko1202/maintmode/internal/storages/maintenances"
)

type Service struct {
    store *maintenances.Store
}

func NewService(store *maintenances.Store) *Service {
    return &Service{store: store}
}

func (s *Service) CreateDraft(ctx context.Context, cmd entity.CreateDraftCommand) (*entity.Maintenance, error) {
    start := time.Now()

    maint := &entity.Maintenance{
        ID:        uuid.New(),
        Title:     cmd.Title,
        Status:    entity.StatusDraft,
        StartTime: cmd.StartTime,
        EndTime:   cmd.EndTime,
        CreatedAt: time.Now(),
    }

    err := s.store.Create(ctx, maint)
    if err != nil {
        metrics.RecordBusinessOperation("create_draft", "error")
        return nil, err
    }

    // Record metrics
    metrics.RecordMaintenanceCreated("draft")
    metrics.RecordBusinessOperation("create_draft", "success")

    return maint, nil
}

func (s *Service) Start(ctx context.Context, id uuid.UUID) error {
    maint, err := s.store.Get(ctx, id)
    if err != nil {
        metrics.RecordBusinessOperation("start", "error")
        return err
    }

    err = s.store.UpdateStatus(ctx, id, entity.StatusInProgress)
    if err != nil {
        metrics.RecordBusinessOperation("start", "error")
        return err
    }

    // Record transition
    metrics.RecordMaintenanceTransition(string(maint.Status), string(entity.StatusInProgress))
    metrics.RecordBusinessOperation("start", "success")

    return nil
}

func (s *Service) Complete(ctx context.Context, id uuid.UUID) error {
    maint, err := s.store.Get(ctx, id)
    if err != nil {
        metrics.RecordBusinessOperation("complete", "error")
        return err
    }

    err = s.store.UpdateStatus(ctx, id, entity.StatusCompleted)
    if err != nil {
        metrics.RecordBusinessOperation("complete", "error")
        return err
    }

    // Calculate duration
    duration := time.Since(maint.StartTime).Seconds()

    // Record metrics
    metrics.RecordMaintenanceCompleted(duration)
    metrics.RecordMaintenanceTransition(string(maint.Status), string(entity.StatusCompleted))
    metrics.RecordBusinessOperation("complete", "success")

    return nil
}
```

## 5. Testing Example

### metrics/http_test.go

```go
package metrics_test

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/labstack/echo/v4"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/testutil"
    "github.com/stretchr/testify/assert"

    "github.com/ruko1202/maintmode/internal/metrics"
)

func TestPrometheusMiddleware(t *testing.T) {
    e := echo.New()
    e.Use(metrics.PrometheusMiddleware())

    e.GET("/test", func(c echo.Context) error {
        return c.String(http.StatusOK, "test")
    })

    // Make request
    req := httptest.NewRequest(http.MethodGet, "/test", nil)
    rec := httptest.NewRecorder()
    e.ServeHTTP(rec, req)

    // Assert response
    assert.Equal(t, http.StatusOK, rec.Code)

    // Note: In real tests, use test registry to assert metrics
    // For package-level metrics, you'd need to refactor to accept registry
}
```

## Usage

### Starting the Server

```go
package main

import (
    "context"

    "github.com/ruko1202/maintmode/internal/config"
    "github.com/ruko1202/maintmode/internal/server"
)

func main() {
    cfg := config.GetAppConfig()
    db := setupDatabase(cfg)

    // Create API server with metrics
    apiServer := server.NewAPIServer(cfg.APIServer, db)

    // Start server
    if err := apiServer.Start(context.Background()); err != nil {
        panic(err)
    }
}
```

### Accessing Metrics

```bash
# View metrics
curl http://localhost:8080/metrics

# Example output:
# HELP maintmode_http_requests_total Total number of HTTP requests
# TYPE maintmode_http_requests_total counter
maintmode_http_requests_total{method="GET",path="/api/maintenances",status="200"} 42

# HELP maintmode_http_request_duration_seconds HTTP request duration in seconds
# TYPE maintmode_http_request_duration_seconds histogram
maintmode_http_request_duration_seconds_bucket{method="GET",path="/api/maintenances",le="0.001"} 5
maintmode_http_request_duration_seconds_bucket{method="GET",path="/api/maintenances",le="0.01"} 35
maintmode_http_request_duration_seconds_bucket{method="GET",path="/api/maintenances",le="0.1"} 42
...

# HELP maintmode_maintenance_created_total Total number of maintenances created
# TYPE maintmode_maintenance_created_total counter
maintmode_maintenance_created_total{status="draft"} 10
```

## Next Steps

1. **Configure Prometheus** to scrape the `/metrics` endpoint
2. **Create Grafana dashboards** to visualize metrics
3. **Set up alerts** for critical metrics (error rates, latency)
4. **Monitor cardinality** in production
5. **Tune histogram buckets** based on actual latency distribution
6. **Add custom collectors** for aggregated business metrics
