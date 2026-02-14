# Performance Optimization for Prometheus Metrics

Best practices and patterns for efficient metrics collection in Go applications.

## Table of Contents
- [Metric Registration Patterns](#metric-registration-patterns)
- [Label Cardinality Management](#label-cardinality-management)
- [Memory Optimization](#memory-optimization)
- [Avoiding Common Pitfalls](#avoiding-common-pitfalls)
- [Benchmarking Metrics](#benchmarking-metrics)

## Metric Registration Patterns

### Pattern 1: Package-Level with promauto

**RECOMMENDED** for most use cases:

```go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // Automatically registered on init
    httpRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total HTTP requests",
        },
        []string{"method", "path", "status"},
    )
)

// Benefits:
// - Simple, clean code
// - Metrics registered once at startup
// - Zero runtime overhead for registration
// - Thread-safe by default
```

### Pattern 2: Explicit Registration

Use when you need control over registry:

```go
package metrics

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
    RequestsTotal *prometheus.CounterVec
    registry      *prometheus.Registry
}

func NewMetrics(registry *prometheus.Registry) *Metrics {
    m := &Metrics{
        registry: registry,
        RequestsTotal: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "http_requests_total",
                Help: "Total HTTP requests",
            },
            []string{"method", "path", "status"},
        ),
    }

    // Explicit registration
    registry.MustRegister(m.RequestsTotal)

    return m
}

// Benefits:
// - Control over registry (useful for testing)
// - Explicit lifecycle management
// - Can use multiple registries
```

### Pattern 3: Lazy Registration

**AVOID** - causes runtime overhead:

```go
// BAD: Creates new metric on each call
func recordRequest(method, path, status string) {
    // This recreates the metric every time!
    counter := prometheus.NewCounterVec(...)
    prometheus.MustRegister(counter) // Panics on second call
    counter.WithLabelValues(method, path, status).Inc()
}
```

## Label Cardinality Management

### Understanding Cardinality

Cardinality = unique combinations of label values

```go
// Low cardinality (good): ~100 combinations
// 5 methods × 20 endpoints = 100 combinations
httpRequests.WithLabelValues(method, path)

// High cardinality (bad): millions of combinations
// Don't do this!
httpRequests.WithLabelValues(method, actualURL, userID, requestID)
```

### Calculating Cardinality

```go
// Example: 3 labels
// method: 5 values (GET, POST, PUT, DELETE, PATCH)
// path: 20 values (unique endpoints)
// status: 10 values (200, 201, 400, 401, 403, 404, 500, 502, 503, 504)
//
// Total cardinality: 5 × 20 × 10 = 1,000 time series
// Memory: ~1,000 × 3KB = 3MB (rough estimate)
```

### Cardinality Guidelines

**Safe limits:**
- **< 1,000 combinations**: Generally safe
- **1,000 - 10,000**: Monitor carefully
- **> 10,000**: High risk, avoid if possible

### Managing Cardinality

**DO:**
```go
// Use route patterns, not actual URLs
path := c.Path() // "/api/users/:id" ✓

// Aggregate status codes
statusClass := fmt.Sprintf("%dxx", status/100) // "2xx", "4xx", "5xx" ✓

// Use bounded enums
operation := "select" // select, insert, update, delete ✓
```

**DON'T:**
```go
// Use actual URLs with IDs
path := c.Request().URL.Path // "/api/users/12345" ✗

// Use individual status codes as labels (unless needed)
status := "404" // When you have many endpoints ✗

// Use unbounded values
userID := "user-123456" // Unbounded ✗
requestID := uuid.New().String() // Unbounded ✗
```

### Cardinality Monitoring

Monitor your metrics cardinality:

```go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    metricsCardinality = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "prometheus_metrics_cardinality",
            Help: "Number of time series per metric",
        },
        []string{"metric_name"},
    )
)

// Collect cardinality periodically
func CollectCardinality() {
    metrics, err := prometheus.DefaultGatherer.Gather()
    if err != nil {
        return
    }

    for _, mf := range metrics {
        name := mf.GetName()
        cardinality := len(mf.GetMetric())
        metricsCardinality.WithLabelValues(name).Set(float64(cardinality))
    }
}
```

## Memory Optimization

### Histogram Bucket Selection

Choose buckets carefully - each bucket creates a time series:

```go
// Too many buckets (bad): 20 buckets = 20 time series per label combination
Buckets: prometheus.LinearBuckets(0, 0.1, 20) // 0, 0.1, 0.2, ..., 1.9

// Appropriate buckets (good): 6 buckets = 6 time series
Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1.0, 5.0}

// Consider your distribution:
// - Fast operations: []float64{0.001, 0.01, 0.1, 0.5, 1.0}
// - Medium operations: []float64{0.01, 0.1, 0.5, 1.0, 5.0, 10.0}
// - Slow operations: []float64{1, 5, 10, 30, 60, 300}
```

### Metric Types and Memory

Memory usage per metric type:

```go
// Counter: ~3KB per time series
counter := promauto.NewCounter(...)

// Gauge: ~3KB per time series
gauge := promauto.NewGauge(...)

// Histogram with 6 buckets: ~3KB × (6 buckets + sum + count) = ~24KB per label combination
histogram := promauto.NewHistogram(prometheus.HistogramOpts{
    Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1.0, 5.0}, // 6 buckets
})

// Summary: Higher memory and CPU overhead than Histogram
// AVOID unless you specifically need client-side quantiles
summary := promauto.NewSummary(...)
```

### Memory Estimation

```go
// Example calculation:
// Metric: http_request_duration_seconds (Histogram)
// Labels: method (5), path (20)
// Buckets: 6
//
// Cardinality: 5 × 20 = 100 label combinations
// Time series per combination: 6 buckets + sum + count = 8
// Total time series: 100 × 8 = 800
// Memory: 800 × 3KB = 2.4MB
```

## Avoiding Common Pitfalls

### Pitfall 1: Label Value Creation in Hot Path

**BAD** - Creates garbage:
```go
// Creates new string on every request
status := fmt.Sprintf("%d", c.Response().Status) // Allocates memory
httpRequests.WithLabelValues(method, path, status).Inc()
```

**GOOD** - Use interned strings:
```go
// Use pre-allocated strings
var statusCodes = map[int]string{
    200: "200",
    201: "201",
    400: "400",
    404: "404",
    500: "500",
}

status := statusCodes[c.Response().Status]
if status == "" {
    status = strconv.Itoa(c.Response().Status) // Fallback
}
httpRequests.WithLabelValues(method, path, status).Inc()
```

### Pitfall 2: Recording Metrics After Error

**BAD** - Misses errors:
```go
func handler(c echo.Context) error {
    err := processRequest(c)
    if err != nil {
        return err // Metrics not recorded!
    }

    // Metrics only recorded on success
    httpRequests.WithLabelValues(...).Inc()
    return nil
}
```

**GOOD** - Record all requests:
```go
func handler(c echo.Context) error {
    err := processRequest(c)

    // Always record metrics (use defer if needed)
    status := "200"
    if err != nil {
        status = "500"
    }
    httpRequests.WithLabelValues(method, path, status).Inc()

    return err
}
```

### Pitfall 3: Not Reusing Label Value Slices

**BAD** - Allocates on every call:
```go
for i := 0; i < 1000000; i++ {
    // New slice allocated each time
    counter.WithLabelValues("GET", "/api/users").Inc()
}
```

**GOOD** - Cache metric with labels:
```go
// Cache the labeled metric
getUsersCounter := counter.WithLabelValues("GET", "/api/users")

for i := 0; i < 1000000; i++ {
    // No allocation
    getUsersCounter.Inc()
}
```

### Pitfall 4: Using Summary Instead of Histogram

**BAD** - Higher overhead:
```go
// Summary calculates quantiles on client side
// Higher CPU and memory usage
summary := promauto.NewSummary(
    prometheus.SummaryOpts{
        Name:       "http_duration",
        Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
    },
)
```

**GOOD** - Use Histogram:
```go
// Histogram is more efficient
// Quantiles calculated on server side (Prometheus/Grafana)
histogram := promauto.NewHistogram(
    prometheus.HistogramOpts{
        Name:    "http_duration_seconds",
        Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1.0, 5.0},
    },
)
```

### Pitfall 5: Recording High-Frequency Events

**BAD** - Too many observations:
```go
// Recording every byte read/written
for {
    n, err := reader.Read(buf)
    bytesRead.Add(float64(n)) // Called millions of times!
}
```

**GOOD** - Aggregate before recording:
```go
totalBytes := 0
for {
    n, err := reader.Read(buf)
    totalBytes += n
    if err == io.EOF {
        break
    }
}
bytesRead.Add(float64(totalBytes)) // Called once per operation
```

## Benchmarking Metrics

Test metrics performance impact:

```go
package metrics_test

import (
    "testing"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    testCounter = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "test_counter",
            Help: "Test counter",
        },
        []string{"label1", "label2"},
    )
)

func BenchmarkCounterInc(b *testing.B) {
    counter := testCounter.WithLabelValues("value1", "value2")

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        counter.Inc()
    }
}

func BenchmarkCounterWithLabels(b *testing.B) {
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        testCounter.WithLabelValues("value1", "value2").Inc()
    }
}

func BenchmarkHistogramObserve(b *testing.B) {
    histogram := promauto.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "test_histogram",
            Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1.0, 5.0},
        },
    )

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        histogram.Observe(0.1)
    }
}

// Typical results:
// BenchmarkCounterInc-8             50000000    25 ns/op    0 B/op   0 allocs/op
// BenchmarkCounterWithLabels-8      10000000   150 ns/op   48 B/op   1 allocs/op
// BenchmarkHistogramObserve-8       20000000    80 ns/op    0 B/op   0 allocs/op
```

## Performance Best Practices Summary

1. **Use `promauto`** for package-level metrics (zero registration overhead)
2. **Keep cardinality low** (< 1,000 combinations ideal)
3. **Choose appropriate buckets** (fewer is better, 5-7 buckets typical)
4. **Use route patterns** for path labels, not actual URLs
5. **Prefer Histogram over Summary** (lower overhead)
6. **Cache labeled metrics** in hot paths
7. **Use interned strings** for label values
8. **Aggregate before recording** high-frequency events
9. **Monitor cardinality** in production
10. **Benchmark critical paths** to measure overhead
11. **Record all requests** (success and error) consistently
12. **Use status classes** (2xx, 4xx, 5xx) to reduce cardinality when appropriate

## Memory Usage Rules of Thumb

- **Base metric**: ~3KB per time series
- **Counter/Gauge**: 1 time series per label combination
- **Histogram**: (buckets + 2) time series per label combination
- **Target total**: < 10,000 active time series per process
- **Safe total**: < 100,000 time series per Prometheus instance

## When to Worry About Performance

**Don't optimize prematurely** - metrics are typically very cheap:
- **< 100 requests/sec**: No performance concerns
- **100-1,000 requests/sec**: Monitor, but likely fine
- **> 1,000 requests/sec**: Follow best practices carefully
- **> 10,000 requests/sec**: Benchmark and optimize

Remember: Metrics overhead is usually < 1% of total request time.
