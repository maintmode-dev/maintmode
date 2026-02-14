# Prometheus Metrics Quick Reference

Essential patterns and snippets for rapid implementation.

## Metric Types Cheat Sheet

| Type | Use Case | Example | Methods |
|------|----------|---------|---------|
| Counter | Counts events | requests, errors, items created | `Inc()`, `Add(n)` |
| Gauge | Current value | connections, queue size, temperature | `Set()`, `Inc()`, `Dec()`, `Add()`, `Sub()` |
| Histogram | Distribution | latencies, request sizes | `Observe()` |
| Summary | Quantiles (client) | latencies (rarely used) | `Observe()` |

## Quick Start Snippets

### 1. Basic Counter

```go
var requestsTotal = promauto.NewCounterVec(
    prometheus.CounterOpts{
        Name: "http_requests_total",
        Help: "Total HTTP requests",
    },
    []string{"method", "status"},
)

// Use it
requestsTotal.WithLabelValues("GET", "200").Inc()
```

### 2. Basic Histogram

```go
var requestDuration = promauto.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "http_request_duration_seconds",
        Help:    "HTTP request duration",
        Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1.0, 5.0},
    },
    []string{"method", "path"},
)

// Use it
start := time.Now()
// ... do work ...
requestDuration.WithLabelValues("GET", "/api/users").Observe(time.Since(start).Seconds())
```

### 3. Basic Gauge

```go
var activeConnections = promauto.NewGauge(
    prometheus.GaugeOpts{
        Name: "db_connections_active",
        Help: "Active database connections",
    },
)

// Use it
activeConnections.Inc()        // +1
activeConnections.Dec()        // -1
activeConnections.Set(10)      // =10
activeConnections.Add(5)       // +5
activeConnections.Sub(3)       // -3
```

### 4. Expose /metrics Endpoint

```go
import (
    "github.com/labstack/echo/v4"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
    e := echo.New()
    e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
    e.Start(":8080")
}
```

## Common Bucket Configurations

```go
// Fast operations (< 100ms)
Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5}

// Web API (< 1s)
Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1.0, 5.0}

// Database queries
Buckets: []float64{0.0001, 0.001, 0.01, 0.1, 0.5, 1.0}

// Background jobs (minutes)
Buckets: []float64{1, 5, 10, 30, 60, 300}

// Use Prometheus defaults
Buckets: prometheus.DefBuckets // [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]

// Exponential buckets
Buckets: prometheus.ExponentialBuckets(100, 10, 8) // 100, 1000, 10000, ...
```

## Naming Conventions

```go
// ✓ Good names
"http_requests_total"                  // Counter with _total suffix
"http_request_duration_seconds"        // Histogram with unit
"db_connections_active"                // Gauge, descriptive
"maintenance_created_total"            // Business metric

// ✗ Bad names
"httpRequests"                         // Use snake_case, not camelCase
"request_duration_ms"                  // Use base units (seconds, not milliseconds)
"requests"                             // Missing _total suffix for counter
"db_query_time"                        // Ambiguous (time in what unit?)
```

## Label Best Practices

```go
// ✓ Good labels (low cardinality)
[]string{"method", "status", "path"}   // Bounded values
method := "GET"                        // 5-10 methods
status := "200"                        // ~10 status codes
path := c.Path()                       // Route pattern: "/api/users/:id"

// ✗ Bad labels (high cardinality)
[]string{"user_id", "request_id"}      // Unbounded
path := c.Request().URL.Path           // Actual path: "/api/users/123456"
timestamp := time.Now().String()       // Unique value per request
```

## Echo Middleware Pattern

```go
package metrics

import (
    "strconv"
    "time"

    "github.com/labstack/echo/v4"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    httpRequests = promauto.NewCounterVec(...)
    httpDuration = promauto.NewHistogramVec(...)
)

func PrometheusMiddleware() echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            start := time.Now()
            err := next(c)

            duration := time.Since(start).Seconds()
            status := strconv.Itoa(c.Response().Status)
            method := c.Request().Method
            path := c.Path() // Important: use route path!

            httpRequests.WithLabelValues(method, path, status).Inc()
            httpDuration.WithLabelValues(method, path).Observe(duration)

            return err
        }
    }
}
```

## Database Metrics Pattern

```go
package metrics

type MetricsExecutor struct {
    dbtx.Executor
    operation string
    table     string
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
```

## Testing Pattern

```go
package metrics_test

import (
    "testing"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/testutil"
    "github.com/stretchr/testify/assert"
)

func TestCounter(t *testing.T) {
    // Create test registry
    reg := prometheus.NewRegistry()

    // Create metric
    counter := prometheus.NewCounter(prometheus.CounterOpts{
        Name: "test_counter",
        Help: "Test counter",
    })
    reg.MustRegister(counter)

    // Use metric
    counter.Inc()
    counter.Inc()

    // Assert
    assert.Equal(t, 2.0, testutil.ToFloat64(counter))
}
```

## Common Patterns

### Pattern 1: Timing Operations

```go
func TimedOperation() {
    start := time.Now()
    defer func() {
        duration := time.Since(start).Seconds()
        operationDuration.Observe(duration)
    }()

    // ... do work ...
}
```

### Pattern 2: Counting with Error Handling

```go
func Operation() error {
    err := doWork()

    if err != nil {
        operationErrors.Inc()
        return err
    }

    operationSuccess.Inc()
    return nil
}
```

### Pattern 3: Tracking Active Operations

```go
func Operation() {
    activeOps.Inc()
    defer activeOps.Dec()

    // ... do work ...
}
```

### Pattern 4: Namespace Convention

```go
var (
    httpRequests = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "maintmode",          // App name
            Subsystem: "http",               // Component
            Name:      "requests_total",     // Metric name
            // Result: maintmode_http_requests_total
        },
        []string{"method", "status"},
    )
)
```

## Prometheus Queries (PromQL)

```promql
# Request rate (requests per second)
rate(http_requests_total[5m])

# Success rate
sum(rate(http_requests_total{status=~"2.."}[5m])) / sum(rate(http_requests_total[5m]))

# 95th percentile latency
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# Average latency
rate(http_request_duration_seconds_sum[5m]) / rate(http_request_duration_seconds_count[5m])

# Error rate
rate(http_requests_total{status=~"5.."}[5m])

# Active connections
db_connections_active

# Current maintenances by status
sum by (status) (maintenances_current)
```

## Grafana Dashboard Panels

```json
{
  "title": "Request Rate",
  "targets": [{
    "expr": "sum(rate(maintmode_http_requests_total[5m])) by (method, path)"
  }]
}

{
  "title": "Latency (95th percentile)",
  "targets": [{
    "expr": "histogram_quantile(0.95, sum(rate(maintmode_http_request_duration_seconds_bucket[5m])) by (le, method, path))"
  }]
}

{
  "title": "Error Rate",
  "targets": [{
    "expr": "sum(rate(maintmode_http_requests_total{status=~\"5..\"}[5m]))"
  }]
}
```

## Debugging Tips

### Check Metric Registration

```go
// List all registered metrics
metrics, _ := prometheus.DefaultGatherer.Gather()
for _, mf := range metrics {
    fmt.Println(mf.GetName())
}
```

### Check Cardinality

```bash
# Count time series per metric
curl -s http://localhost:8080/metrics | grep "^maintmode_" | wc -l

# View all label combinations
curl -s http://localhost:8080/metrics | grep "maintmode_http_requests_total"
```

### Common Errors

```go
// ERROR: Duplicate metric registration
// Solution: Use promauto or register only once

// ERROR: High cardinality
// Solution: Check label values, use route patterns not actual URLs

// ERROR: Labels inconsistent
// Solution: Always provide same labels in same order
```

## Performance Tips

1. **Cache labeled metrics** in hot paths:
   ```go
   // Good: Cache the metric with labels
   getUsersMetric := counter.WithLabelValues("GET", "/api/users")
   getUsersMetric.Inc()
   ```

2. **Use promauto** for package-level metrics
3. **Keep cardinality < 1000** combinations
4. **Choose 5-7 histogram buckets** (not 20+)
5. **Use route paths** not actual URLs
6. **Prefer Histogram over Summary**

## Quick Wins for MaintMode

```go
// 1. HTTP metrics (already implemented)
e.Use(metrics.PrometheusMiddleware())

// 2. Database metrics
executor := metrics.NewMetricsExecutor(db, "select", "maintenances")

// 3. Business metrics
metrics.RecordMaintenanceCreated("draft")
metrics.RecordConflictDetected("resource")

// 4. Expose endpoint
e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

// 5. Start pool collector
metrics.StartPoolMetricsCollector(db, 15*time.Second)
```
