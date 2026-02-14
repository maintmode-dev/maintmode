---
name: prometheus-metrics
description: Metrics collection patterns with Prometheus for Go applications. Use this skill when implementing production monitoring, adding metrics to Echo HTTP servers, instrumenting database queries, tracking business metrics (maintenance operations, conflict detection), designing custom Prometheus metrics, exposing /metrics endpoints, configuring metric labels and naming conventions, integrating with Echo v4 middleware, or optimizing metrics performance in Go applications.
license: MIT
metadata:
  category: development
  source:
    repository: project-specific
    path: prometheus-metrics
---

# Prometheus Metrics Skill

## Overview

Implement production-grade Prometheus metrics for Go applications using Echo v4 framework with PostgreSQL. Track API performance, database queries, and business domain metrics.

## Key Concepts

### Metric Types

**Counter**: Monotonically increasing value (requests, errors, events)
```go
httpRequestsTotal := promauto.NewCounterVec(
    prometheus.CounterOpts{
        Name: "http_requests_total",
        Help: "Total number of HTTP requests",
    },
    []string{"method", "path", "status"},
)
```

**Gauge**: Value that can go up or down (connections, goroutines, queue size)
```go
activeConnections := promauto.NewGauge(prometheus.GaugeOpts{
    Name: "db_connections_active",
    Help: "Number of active database connections",
})
```

**Histogram**: Samples observations and counts them in buckets (latencies, sizes)
```go
httpDuration := promauto.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "http_request_duration_seconds",
        Help:    "HTTP request latency",
        Buckets: prometheus.DefBuckets, // or custom buckets
    },
    []string{"method", "path"},
)
```

**Summary**: Similar to Histogram but calculates quantiles on client side (not recommended for most cases)

### Naming Conventions

Follow Prometheus best practices:
- Use snake_case: `http_requests_total`
- Suffix with unit: `_seconds`, `_bytes`, `_total`
- Counter suffix: `_total` or `_count`
- Base unit: seconds (not milliseconds), bytes (not kilobytes)
- Labels for dimensions: `{method="GET", status="200"}`

**Examples:**
- `http_request_duration_seconds` (Histogram)
- `http_requests_total` (Counter)
- `db_query_duration_seconds` (Histogram)
- `maintenances_created_total` (Counter)
- `maintenance_conflicts_detected_total` (Counter)

### Label Best Practices

**DO:**
- Use labels for dimensions: method, status, operation
- Keep cardinality low (< 1000 unique combinations)
- Use consistent label names across metrics

**DON'T:**
- Use high-cardinality labels: user_id, request_id, timestamp
- Include unbounded values: URLs with IDs
- Use labels for values that change frequently

## Quick Start

### 1. Dependencies

```bash
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promauto
go get github.com/prometheus/client_golang/prometheus/promhttp
```

### 2. Expose /metrics Endpoint

```go
import (
    "github.com/labstack/echo/v4"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

func RegisterMetricsEndpoint(e *echo.Echo) {
    e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
}
```

### 3. Basic HTTP Middleware

```go
func PrometheusMiddleware() echo.MiddlewareFunc {
    requestsTotal := promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total HTTP requests",
        },
        []string{"method", "path", "status"},
    )

    requestDuration := promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP request latency",
            Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1.0, 5.0},
        },
        []string{"method", "path"},
    )

    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            start := time.Now()

            err := next(c)

            duration := time.Since(start).Seconds()
            status := strconv.Itoa(c.Response().Status)
            method := c.Request().Method
            path := c.Path() // use route path, not actual path

            requestsTotal.WithLabelValues(method, path, status).Inc()
            requestDuration.WithLabelValues(method, path).Observe(duration)

            return err
        }
    }
}
```

## Main Components

### HTTP Metrics

Echo middleware for tracking HTTP requests. See [references/http-metrics.md](references/http-metrics.md) for:
- Complete middleware implementation
- Request/response size tracking
- Status code distribution
- Integration with existing logging

### Database Metrics

PostgreSQL query instrumentation. See [references/database-metrics.md](references/database-metrics.md) for:
- Query duration tracking
- Connection pool metrics
- Query error tracking
- Custom database wrapper patterns

### Business Metrics

Domain-specific metrics for MaintMode. See [references/business-metrics.md](references/business-metrics.md) for:
- Maintenance lifecycle metrics
- Conflict detection tracking
- Resource utilization metrics
- Custom collector patterns

### Performance Optimization

Efficient metrics collection. See [references/performance.md](references/performance.md) for:
- Metric registration patterns
- Label cardinality management
- Memory optimization
- Avoiding common pitfalls

### Testing Strategies

Testing metrics in Go. See [references/testing.md](references/testing.md) for:
- Unit testing metrics
- Integration testing with test registries
- Mocking collectors
- Validating metric output

## Integration Patterns

### With Echo Framework

```go
func NewAPIServer(cfg config.HTTPServer) *server {
    e := echo.New()

    // Metrics middleware before logging
    e.Use(PrometheusMiddleware())
    e.Use(middleware.RequestLogger())

    // Expose metrics endpoint
    e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

    return &server{cfg: cfg, e: e}
}
```

### With Zap Logging

Coordinate metrics with structured logging:
```go
xlog.Info(ctx, "maintenance_created",
    zap.String("id", maintID),
    zap.Duration("duration", time.Since(start)),
)
maintenancesCreated.WithLabelValues("draft").Inc()
```

### With Database Transactions

```go
func (s *Store) WithMetrics(query string) dbtx.Executor {
    return &metricsExecutor{
        Executor: s.db,
        query:    query,
    }
}

type metricsExecutor struct {
    dbtx.Executor
    query string
}

func (m *metricsExecutor) QueryContext(ctx context.Context, ...) error {
    start := time.Now()
    err := m.Executor.QueryContext(ctx, ...)

    duration := time.Since(start).Seconds()
    dbQueryDuration.WithLabelValues(m.query).Observe(duration)

    if err != nil {
        dbQueryErrors.WithLabelValues(m.query).Inc()
    }

    return err
}
```

## Histogram Bucket Configuration

Choose buckets based on expected latency distribution:

**API endpoints** (typical web service):
```go
Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1.0, 5.0}
// 1ms, 10ms, 100ms, 500ms, 1s, 5s
```

**Database queries** (fast OLTP):
```go
Buckets: []float64{0.0001, 0.001, 0.01, 0.1, 0.5, 1.0}
// 0.1ms, 1ms, 10ms, 100ms, 500ms, 1s
```

**Background jobs** (long-running):
```go
Buckets: []float64{1, 5, 10, 30, 60, 300}
// 1s, 5s, 10s, 30s, 1min, 5min
```

## Best Practices

1. **Initialize metrics at package level** using `promauto`
2. **Use route paths** (`c.Path()`) not actual paths for labels
3. **Keep label cardinality low** (< 1000 combinations)
4. **Histogram over Summary** for most use cases
5. **Choose appropriate buckets** for your latency distribution
6. **Coordinate with logging** - metrics for aggregation, logs for debugging
7. **Test metrics** using `testutil` package
8. **Document metrics** in comments and dashboards
9. **Monitor metrics cardinality** in production
10. **Use separate registry** for testing

## Common Patterns

### Pattern 1: Package-level Metrics

```go
package metrics

var (
    httpRequestsTotal = promauto.NewCounterVec(...)
    httpDuration = promauto.NewHistogramVec(...)
)

func HTTPMiddleware() echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // use httpRequestsTotal, httpDuration
        }
    }
}
```

### Pattern 2: Metrics Struct

```go
type Metrics struct {
    RequestsTotal *prometheus.CounterVec
    Duration      *prometheus.HistogramVec
}

func NewMetrics() *Metrics {
    return &Metrics{
        RequestsTotal: promauto.NewCounterVec(...),
        Duration:      promauto.NewHistogramVec(...),
    }
}

func (m *Metrics) Middleware() echo.MiddlewareFunc { ... }
```

### Pattern 3: Custom Collector

```go
type maintenanceCollector struct {
    store Store
}

func (c *maintenanceCollector) Describe(ch chan<- *prometheus.Desc) {
    ch <- prometheus.NewDesc(...)
}

func (c *maintenanceCollector) Collect(ch chan<- prometheus.Metric) {
    stats := c.store.GetStats()
    ch <- prometheus.MustNewConstMetric(...)
}

prometheus.MustRegister(&maintenanceCollector{store: store})
```

## Example: Complete Implementation

See [references/complete-example.md](references/complete-example.md) for a full working example integrating:
- HTTP middleware with Echo v4
- Database query metrics
- Business metrics for maintenance operations
- Testing setup with test registry

## Official Resources

- **Prometheus Go Client**: https://github.com/prometheus/client_golang
- **Prometheus Best Practices**: https://prometheus.io/docs/practices/naming/
- **Prometheus Instrumentation**: https://prometheus.io/docs/practices/instrumentation/
- **Echo Prometheus Middleware**: https://github.com/labstack/echo-contrib/tree/master/prometheus

## Related Skills

- **echo-framework**: Echo v4 web framework patterns
- **zap-logging**: Structured logging with Zap
- **golang-backend**: Go backend development best practices
- **docker-go-apps**: Containerization with metrics exposure
