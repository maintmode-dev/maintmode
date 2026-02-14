# Testing Prometheus Metrics

Comprehensive guide for testing Prometheus metrics in Go applications.

## Table of Contents
- [Unit Testing Metrics](#unit-testing-metrics)
- [Integration Testing](#integration-testing)
- [Test Registries](#test-registries)
- [Mocking Collectors](#mocking-collectors)
- [Validating Output](#validating-output)

## Unit Testing Metrics

### Testing Counter Increments

```go
package metrics_test

import (
    "testing"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/testutil"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestCounterIncrement(t *testing.T) {
    // Create test registry
    reg := prometheus.NewRegistry()

    // Create counter with test registry
    counter := prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "test_requests_total",
            Help: "Test requests counter",
        },
        []string{"method", "status"},
    )
    reg.MustRegister(counter)

    // Increment counter
    counter.WithLabelValues("GET", "200").Inc()
    counter.WithLabelValues("GET", "200").Inc()
    counter.WithLabelValues("POST", "201").Inc()

    // Assert counter values
    assert.Equal(t, 2.0, testutil.ToFloat64(counter.WithLabelValues("GET", "200")))
    assert.Equal(t, 1.0, testutil.ToFloat64(counter.WithLabelValues("POST", "201")))
    assert.Equal(t, 0.0, testutil.ToFloat64(counter.WithLabelValues("DELETE", "404")))
}
```

### Testing Histogram Observations

```go
func TestHistogramObservations(t *testing.T) {
    reg := prometheus.NewRegistry()

    histogram := prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "test_duration_seconds",
            Help:    "Test duration histogram",
            Buckets: []float64{0.1, 0.5, 1.0, 5.0},
        },
    )
    reg.MustRegister(histogram)

    // Record observations
    histogram.Observe(0.05) // < 0.1
    histogram.Observe(0.3)  // 0.1 - 0.5
    histogram.Observe(0.7)  // 0.5 - 1.0
    histogram.Observe(2.0)  // 1.0 - 5.0

    // Assert histogram metrics
    count := testutil.ToFloat64(histogram)
    assert.Equal(t, 4.0, count, "should have 4 observations")

    // Get detailed metrics
    metric := &dto.Metric{}
    require.NoError(t, histogram.Write(metric))

    h := metric.GetHistogram()
    assert.Equal(t, uint64(4), h.GetSampleCount())
    assert.InDelta(t, 3.05, h.GetSampleSum(), 0.01) // 0.05 + 0.3 + 0.7 + 2.0

    // Check buckets
    buckets := h.GetBucket()
    assert.Equal(t, uint64(1), buckets[0].GetCumulativeCount()) // <= 0.1
    assert.Equal(t, uint64(2), buckets[1].GetCumulativeCount()) // <= 0.5
    assert.Equal(t, uint64(3), buckets[2].GetCumulativeCount()) // <= 1.0
    assert.Equal(t, uint64(4), buckets[3].GetCumulativeCount()) // <= 5.0
}
```

### Testing Gauge Changes

```go
func TestGaugeChanges(t *testing.T) {
    reg := prometheus.NewRegistry()

    gauge := prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "test_connections",
            Help: "Test connections gauge",
        },
    )
    reg.MustRegister(gauge)

    // Set and modify gauge
    gauge.Set(10)
    assert.Equal(t, 10.0, testutil.ToFloat64(gauge))

    gauge.Inc()
    assert.Equal(t, 11.0, testutil.ToFloat64(gauge))

    gauge.Dec()
    assert.Equal(t, 10.0, testutil.ToFloat64(gauge))

    gauge.Add(5)
    assert.Equal(t, 15.0, testutil.ToFloat64(gauge))

    gauge.Sub(3)
    assert.Equal(t, 12.0, testutil.ToFloat64(gauge))
}
```

## Integration Testing

### Testing HTTP Middleware Metrics

```go
package middlewares_test

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/labstack/echo/v4"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/testutil"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/ruko1202/maintmode/internal/metrics"
)

func TestHTTPMetricsMiddleware(t *testing.T) {
    // Setup test registry
    reg := prometheus.NewRegistry()

    // Create metrics with test registry
    requestsTotal := prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total HTTP requests",
        },
        []string{"method", "path", "status"},
    )
    reg.MustRegister(requestsTotal)

    requestDuration := prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP request duration",
            Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1.0},
        },
        []string{"method", "path"},
    )
    reg.MustRegister(requestDuration)

    // Create middleware using test metrics
    middleware := func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            err := next(c)

            method := c.Request().Method
            path := c.Path()
            status := strconv.Itoa(c.Response().Status)

            requestsTotal.WithLabelValues(method, path, status).Inc()
            requestDuration.WithLabelValues(method, path).Observe(0.1)

            return err
        }
    }

    // Setup Echo
    e := echo.New()
    e.Use(middleware)
    e.GET("/test", func(c echo.Context) error {
        return c.String(http.StatusOK, "test")
    })

    // Make request
    req := httptest.NewRequest(http.MethodGet, "/test", nil)
    rec := httptest.NewRecorder()
    e.ServeHTTP(rec, req)

    // Assert response
    assert.Equal(t, http.StatusOK, rec.Code)

    // Assert metrics
    count := testutil.ToFloat64(requestsTotal.WithLabelValues("GET", "/test", "200"))
    assert.Equal(t, 1.0, count)

    durationCount := testutil.ToFloat64(requestDuration.WithLabelValues("GET", "/test"))
    assert.Equal(t, 1.0, durationCount)
}
```

### Testing Service Layer Metrics

```go
package maint_test

import (
    "context"
    "testing"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/testutil"
    "github.com/stretchr/testify/assert"

    "github.com/ruko1202/maintmode/internal/entity"
    "github.com/ruko1202/maintmode/internal/services/maint"
    "github.com/ruko1202/maintmode/internal/metrics"
)

func TestMaintenanceCreationMetrics(t *testing.T) {
    // Setup test registry
    reg := prometheus.NewRegistry()

    maintenancesCreated := prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "maintenances_created_total",
            Help: "Total maintenances created",
        },
        []string{"status"},
    )
    reg.MustRegister(maintenancesCreated)

    // Setup service with test store
    store := setupTestStore(t)
    service := maint.NewService(store)

    // Record creation
    ctx := context.Background()
    _, err := service.CreateDraft(ctx, entity.CreateDraftCommand{
        Title:     "Test Maintenance",
        Resources: []string{"server-1", "server-2"},
    })
    require.NoError(t, err)

    // Record metric (in real code, this is in service)
    maintenancesCreated.WithLabelValues("draft").Inc()

    // Assert metrics
    count := testutil.ToFloat64(maintenancesCreated.WithLabelValues("draft"))
    assert.Equal(t, 1.0, count)
}
```

## Test Registries

### Creating Test Registry

```go
package metrics_test

import (
    "testing"

    "github.com/prometheus/client_golang/prometheus"
)

// createTestRegistry creates a clean registry for testing
func createTestRegistry(t *testing.T) *prometheus.Registry {
    t.Helper()
    return prometheus.NewRegistry()
}

// createTestMetrics creates metrics with test registry
func createTestMetrics(t *testing.T, reg *prometheus.Registry) (*Metrics, error) {
    t.Helper()

    m := &Metrics{
        RequestsTotal: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "http_requests_total",
                Help: "Total HTTP requests",
            },
            []string{"method", "path", "status"},
        ),
    }

    if err := reg.Register(m.RequestsTotal); err != nil {
        return nil, err
    }

    return m, nil
}
```

### Testing with Table Tests

```go
func TestMetricsWithTableTests(t *testing.T) {
    tests := []struct {
        name           string
        method         string
        path           string
        status         string
        expectedCount  float64
    }{
        {
            name:          "GET request",
            method:        "GET",
            path:          "/api/users",
            status:        "200",
            expectedCount: 1.0,
        },
        {
            name:          "POST request",
            method:        "POST",
            path:          "/api/users",
            status:        "201",
            expectedCount: 1.0,
        },
        {
            name:          "error request",
            method:        "GET",
            path:          "/api/users",
            status:        "500",
            expectedCount: 1.0,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Fresh registry for each test
            reg := prometheus.NewRegistry()
            counter := prometheus.NewCounterVec(
                prometheus.CounterOpts{
                    Name: "test_requests_total",
                    Help: "Test counter",
                },
                []string{"method", "path", "status"},
            )
            reg.MustRegister(counter)

            // Record metric
            counter.WithLabelValues(tt.method, tt.path, tt.status).Inc()

            // Assert
            count := testutil.ToFloat64(counter.WithLabelValues(tt.method, tt.path, tt.status))
            assert.Equal(t, tt.expectedCount, count)
        })
    }
}
```

## Mocking Collectors

### Mock Collector for Testing

```go
package metrics_test

import (
    "github.com/prometheus/client_golang/prometheus"
)

type mockCollector struct {
    descCalled    bool
    collectCalled bool
    metrics       []prometheus.Metric
}

func newMockCollector() *mockCollector {
    return &mockCollector{
        metrics: make([]prometheus.Metric, 0),
    }
}

func (m *mockCollector) Describe(ch chan<- *prometheus.Desc) {
    m.descCalled = true
}

func (m *mockCollector) Collect(ch chan<- prometheus.Metric) {
    m.collectCalled = true
    for _, metric := range m.metrics {
        ch <- metric
    }
}

func (m *mockCollector) AddMetric(metric prometheus.Metric) {
    m.metrics = append(m.metrics, metric)
}

func TestCustomCollector(t *testing.T) {
    reg := prometheus.NewRegistry()
    mock := newMockCollector()

    reg.MustRegister(mock)

    // Gather metrics (triggers Describe and Collect)
    _, err := reg.Gather()
    require.NoError(t, err)

    assert.True(t, mock.descCalled, "Describe should be called")
    assert.True(t, mock.collectCalled, "Collect should be called")
}
```

## Validating Output

### Testing Metric Output Format

```go
func TestMetricOutput(t *testing.T) {
    reg := prometheus.NewRegistry()

    counter := prometheus.NewCounter(
        prometheus.CounterOpts{
            Name: "test_counter",
            Help: "Test counter",
        },
    )
    reg.MustRegister(counter)

    counter.Inc()
    counter.Inc()

    // Test metric output format
    expected := `
# HELP test_counter Test counter
# TYPE test_counter counter
test_counter 2
`

    err := testutil.CollectAndCompare(counter, strings.NewReader(expected))
    assert.NoError(t, err)
}
```

### Testing Metric Labels

```go
func TestMetricLabels(t *testing.T) {
    reg := prometheus.NewRegistry()

    counter := prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "test_requests_total",
            Help: "Test requests",
        },
        []string{"method", "status"},
    )
    reg.MustRegister(counter)

    counter.WithLabelValues("GET", "200").Add(2)
    counter.WithLabelValues("POST", "201").Inc()

    // Test with expected output
    expected := `
# HELP test_requests_total Test requests
# TYPE test_requests_total counter
test_requests_total{method="GET",status="200"} 2
test_requests_total{method="POST",status="201"} 1
`

    err := testutil.CollectAndCompare(counter, strings.NewReader(expected))
    assert.NoError(t, err)
}
```

### Testing Custom Collectors

```go
package metrics_test

import (
    "context"
    "testing"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/testutil"
    "github.com/stretchr/testify/assert"

    "github.com/ruko1202/maintmode/internal/metrics"
    "github.com/ruko1202/maintmode/internal/storages/maintenances"
)

func TestMaintenanceCollector(t *testing.T) {
    // Setup test store with known data
    store := setupTestStore(t)
    ctx := context.Background()

    // Create test maintenances
    createTestMaintenance(t, store, "draft")
    createTestMaintenance(t, store, "scheduled")
    createTestMaintenance(t, store, "completed")

    // Create collector
    collector := metrics.NewMaintenanceCollector(store)

    // Register with test registry
    reg := prometheus.NewRegistry()
    reg.MustRegister(collector)

    // Gather metrics
    metricFamilies, err := reg.Gather()
    require.NoError(t, err)

    // Assert metrics are present
    assert.NotEmpty(t, metricFamilies)

    // Find specific metric
    var totalMetric *dto.MetricFamily
    for _, mf := range metricFamilies {
        if mf.GetName() == "maintenances_total" {
            totalMetric = mf
            break
        }
    }

    require.NotNil(t, totalMetric, "maintenances_total metric should exist")
    assert.Equal(t, 3.0, totalMetric.GetMetric()[0].GetGauge().GetValue())
}
```

## Testing Metrics in CI/CD

### Metrics Validation Test

```go
package metrics_test

import (
    "testing"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/stretchr/testify/assert"
)

func TestAllMetricsRegistered(t *testing.T) {
    // List all expected metrics
    expectedMetrics := []string{
        "http_requests_total",
        "http_request_duration_seconds",
        "db_query_duration_seconds",
        "maintenances_created_total",
        "maintenance_conflicts_detected_total",
    }

    // Gather all metrics
    metricFamilies, err := prometheus.DefaultGatherer.Gather()
    require.NoError(t, err)

    // Build map of registered metrics
    registered := make(map[string]bool)
    for _, mf := range metricFamilies {
        registered[mf.GetName()] = true
    }

    // Verify all expected metrics are registered
    for _, name := range expectedMetrics {
        assert.True(t, registered[name], "metric %s should be registered", name)
    }
}
```

### Cardinality Test

```go
func TestMetricsCardinality(t *testing.T) {
    // Simulate production load
    reg := prometheus.NewRegistry()
    counter := prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "test_requests_total",
            Help: "Test requests",
        },
        []string{"method", "path", "status"},
    )
    reg.MustRegister(counter)

    // Simulate various requests
    methods := []string{"GET", "POST", "PUT", "DELETE"}
    paths := []string{"/api/users", "/api/posts", "/api/comments"}
    statuses := []string{"200", "201", "400", "404", "500"}

    for _, method := range methods {
        for _, path := range paths {
            for _, status := range statuses {
                counter.WithLabelValues(method, path, status).Inc()
            }
        }
    }

    // Check cardinality
    metricFamilies, err := reg.Gather()
    require.NoError(t, err)

    for _, mf := range metricFamilies {
        if mf.GetName() == "test_requests_total" {
            cardinality := len(mf.GetMetric())
            // 4 methods × 3 paths × 5 statuses = 60 combinations
            assert.Equal(t, 60, cardinality)
            assert.Less(t, cardinality, 1000, "cardinality should be < 1000")
        }
    }
}
```

## Test Utilities

### Helper Functions

```go
package testutil

import (
    "testing"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/testutil"
)

// AssertCounterValue asserts counter value
func AssertCounterValue(t *testing.T, counter prometheus.Counter, expected float64) {
    t.Helper()
    actual := testutil.ToFloat64(counter)
    if actual != expected {
        t.Errorf("counter value: got %v, want %v", actual, expected)
    }
}

// AssertCounterVecValue asserts counter vec value
func AssertCounterVecValue(t *testing.T, counter *prometheus.CounterVec, expected float64, labels ...string) {
    t.Helper()
    actual := testutil.ToFloat64(counter.WithLabelValues(labels...))
    if actual != expected {
        t.Errorf("counter value for %v: got %v, want %v", labels, actual, expected)
    }
}

// GetMetricFamily gets metric family by name
func GetMetricFamily(t *testing.T, reg *prometheus.Registry, name string) *dto.MetricFamily {
    t.Helper()

    metricFamilies, err := reg.Gather()
    if err != nil {
        t.Fatalf("failed to gather metrics: %v", err)
    }

    for _, mf := range metricFamilies {
        if mf.GetName() == name {
            return mf
        }
    }

    return nil
}
```

## Best Practices for Testing Metrics

1. **Use test registries** - Avoid polluting global registry
2. **Test in isolation** - Each test gets fresh registry
3. **Assert concrete values** - Don't just check > 0
4. **Test error cases** - Ensure errors are tracked
5. **Validate cardinality** - Check label combinations don't explode
6. **Test metric output** - Use `CollectAndCompare` for format validation
7. **Mock collectors** - For complex custom collectors
8. **Integration tests** - Test real middleware/service integration
9. **CI validation** - Ensure all expected metrics are registered
10. **Helper functions** - Create reusable test utilities

## Common Testing Patterns

```go
// Pattern 1: Setup/Teardown with test registry
func setupTestMetrics(t *testing.T) (*prometheus.Registry, *Metrics) {
    t.Helper()
    reg := prometheus.NewRegistry()
    metrics := NewMetrics(reg)
    return reg, metrics
}

// Pattern 2: Assert metric increased
func assertMetricIncreased(t *testing.T, before, after float64) {
    t.Helper()
    assert.Greater(t, after, before, "metric should increase")
}

// Pattern 3: Reset metrics after test
func resetMetrics(t *testing.T, reg *prometheus.Registry) {
    t.Helper()
    // Note: Prometheus metrics can't be reset directly
    // Use fresh registry per test instead
}
```
