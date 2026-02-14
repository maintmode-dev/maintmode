# HTTP Metrics with Echo v4

Complete guide for instrumenting Echo HTTP servers with Prometheus metrics.

## Table of Contents
- [Basic Middleware](#basic-middleware)
- [Advanced Middleware](#advanced-middleware)
- [Request/Response Size Tracking](#requestresponse-size-tracking)
- [Status Code Distribution](#status-code-distribution)
- [Integration with Existing Logging](#integration-with-existing-logging)

## Basic Middleware

Simple HTTP metrics middleware for Echo v4:

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
            Name: "http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"method", "path", "status"},
    )

    httpRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP request duration in seconds",
            Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1.0, 5.0},
        },
        []string{"method", "path"},
    )
)

func PrometheusMiddleware() echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            start := time.Now()

            // Execute handler
            err := next(c)

            // Record metrics
            duration := time.Since(start).Seconds()
            status := strconv.Itoa(c.Response().Status)
            method := c.Request().Method
            path := c.Path() // Use route path, not actual URL

            httpRequestsTotal.WithLabelValues(method, path, status).Inc()
            httpRequestDuration.WithLabelValues(method, path).Observe(duration)

            return err
        }
    }
}
```

## Advanced Middleware

Complete implementation with additional metrics:

```go
package metrics

import (
    "strconv"
    "time"

    "github.com/labstack/echo/v4"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

type HTTPMetrics struct {
    requestsTotal   *prometheus.CounterVec
    requestDuration *prometheus.HistogramVec
    requestSize     *prometheus.HistogramVec
    responseSize    *prometheus.HistogramVec
    requestsActive  prometheus.Gauge
}

func NewHTTPMetrics() *HTTPMetrics {
    return &HTTPMetrics{
        requestsTotal: promauto.NewCounterVec(
            prometheus.CounterOpts{
                Name: "http_requests_total",
                Help: "Total number of HTTP requests",
            },
            []string{"method", "path", "status"},
        ),
        requestDuration: promauto.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "http_request_duration_seconds",
                Help:    "HTTP request duration in seconds",
                Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1.0, 5.0},
            },
            []string{"method", "path"},
        ),
        requestSize: promauto.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "http_request_size_bytes",
                Help:    "HTTP request size in bytes",
                Buckets: prometheus.ExponentialBuckets(100, 10, 8), // 100B to ~10MB
            },
            []string{"method", "path"},
        ),
        responseSize: promauto.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "http_response_size_bytes",
                Help:    "HTTP response size in bytes",
                Buckets: prometheus.ExponentialBuckets(100, 10, 8), // 100B to ~10MB
            },
            []string{"method", "path"},
        ),
        requestsActive: promauto.NewGauge(
            prometheus.GaugeOpts{
                Name: "http_requests_active",
                Help: "Number of HTTP requests currently being processed",
            },
        ),
    }
}

func (m *HTTPMetrics) Middleware() echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            start := time.Now()

            // Track active requests
            m.requestsActive.Inc()
            defer m.requestsActive.Dec()

            // Track request size
            reqSize := computeRequestSize(c.Request())

            // Execute handler
            err := next(c)

            // Record metrics
            duration := time.Since(start).Seconds()
            status := strconv.Itoa(c.Response().Status)
            method := c.Request().Method
            path := c.Path()

            m.requestsTotal.WithLabelValues(method, path, status).Inc()
            m.requestDuration.WithLabelValues(method, path).Observe(duration)
            m.requestSize.WithLabelValues(method, path).Observe(float64(reqSize))
            m.responseSize.WithLabelValues(method, path).Observe(float64(c.Response().Size))

            return err
        }
    }
}

func computeRequestSize(r *http.Request) int {
    size := 0
    if r.ContentLength > 0 {
        size = int(r.ContentLength)
    }
    return size
}
```

## Request/Response Size Tracking

Track payload sizes for bandwidth monitoring:

```go
var (
    httpRequestSizeBytes = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_size_bytes",
            Help:    "HTTP request size in bytes",
            Buckets: prometheus.ExponentialBuckets(100, 10, 8),
            // Buckets: 100, 1000, 10000, 100000, 1000000, etc.
        },
        []string{"method", "path"},
    )

    httpResponseSizeBytes = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_response_size_bytes",
            Help:    "HTTP response size in bytes",
            Buckets: prometheus.ExponentialBuckets(100, 10, 8),
        },
        []string{"method", "path"},
    )
)

// In middleware:
reqSize := int(c.Request().ContentLength)
if reqSize > 0 {
    httpRequestSizeBytes.WithLabelValues(method, path).Observe(float64(reqSize))
}

respSize := c.Response().Size
httpResponseSizeBytes.WithLabelValues(method, path).Observe(float64(respSize))
```

## Status Code Distribution

Track HTTP status codes for error monitoring:

```go
var (
    httpResponsesTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_responses_total",
            Help: "Total HTTP responses by status code",
        },
        []string{"status_class"}, // 2xx, 3xx, 4xx, 5xx
    )
)

func statusClass(status int) string {
    switch {
    case status >= 200 && status < 300:
        return "2xx"
    case status >= 300 && status < 400:
        return "3xx"
    case status >= 400 && status < 500:
        return "4xx"
    case status >= 500 && status < 600:
        return "5xx"
    default:
        return "unknown"
    }
}

// In middleware:
status := c.Response().Status
httpResponsesTotal.WithLabelValues(statusClass(status)).Inc()
```

## Integration with Existing Logging

Coordinate metrics with zap logging used in MaintMode:

```go
package middlewares

import (
    "strconv"
    "time"

    "github.com/labstack/echo/v4"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "github.com/ruko1202/xlog"
    "go.uber.org/zap"
)

var (
    httpMetrics = metrics.NewHTTPMetrics()
)

// Add metrics middleware before logging middleware
func BaseMiddlewares() []echo.MiddlewareFunc {
    return []echo.MiddlewareFunc{
        middleware.Recover(),
        middleware.Secure(),
        middleware.RequestIDWithConfig(middleware.RequestIDConfig{
            Generator: xuuid.NewString,
        }),
        httpMetrics.Middleware(), // Metrics BEFORE logging
        RequestLoggingMiddleware(), // Existing logging
        middleware.GzipWithConfig(...),
    }
}

// Enhanced logging middleware with metrics context
func RequestLoggingMiddleware() echo.MiddlewareFunc {
    return middleware.RequestLoggerWithConfig(
        middleware.RequestLoggerConfig{
            // ... existing config ...
            LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
                ctx := c.Request().Context()
                attrs := []zap.Field{
                    zap.String("request", fmt.Sprintf("%s %s", v.Method, v.URI)),
                    zap.Int("status", v.Status),
                    zap.Duration("latency", v.Latency),
                    zap.String("request_id", v.RequestID),
                    // Metrics are already recorded by metrics middleware
                }

                if v.Error != nil {
                    xlog.Error(ctx, "REQUEST_ERROR", append(attrs, zap.Error(v.Error))...)
                    return nil
                }

                xlog.Info(ctx, "REQUEST", attrs...)
                return nil
            },
        },
    )
}
```

## Path Normalization

Normalize paths to avoid high cardinality:

```go
// Use c.Path() for metrics (route pattern with placeholders)
path := c.Path() // "/api/v1/maintenances/:id"

// NOT c.Request().URL.Path (actual path with values)
// Bad: "/api/v1/maintenances/123e4567-e89b-12d3-a456-426614174000"
```

## Middleware Registration

Register metrics middleware in server initialization:

```go
package server

import (
    "github.com/labstack/echo/v4"
    "github.com/prometheus/client_golang/prometheus/promhttp"

    "github.com/ruko1202/maintmode/internal/config/middlewares"
    "github.com/ruko1202/maintmode/internal/metrics"
)

func NewAPIServer(cfg config.HTTPServer) *server {
    e := echo.New()

    // Register base middlewares (includes metrics)
    for _, mw := range middlewares.BaseMiddlewares() {
        e.Use(mw)
    }

    // Expose /metrics endpoint
    e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

    return &server{cfg: cfg, e: e}
}
```

## Testing HTTP Metrics

Test metrics collection in HTTP handlers:

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
)

func TestPrometheusMiddleware(t *testing.T) {
    // Create test registry
    reg := prometheus.NewRegistry()

    // Create metrics with test registry
    requestsTotal := prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Test counter",
        },
        []string{"method", "path", "status"},
    )
    reg.MustRegister(requestsTotal)

    // Setup Echo with middleware
    e := echo.New()
    e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            err := next(c)
            requestsTotal.WithLabelValues(
                c.Request().Method,
                c.Path(),
                strconv.Itoa(c.Response().Status),
            ).Inc()
            return err
        }
    })
    e.GET("/test", func(c echo.Context) error {
        return c.String(http.StatusOK, "test")
    })

    // Make request
    req := httptest.NewRequest(http.MethodGet, "/test", nil)
    rec := httptest.NewRecorder()
    e.ServeHTTP(rec, req)

    // Assert metrics
    assert.Equal(t, http.StatusOK, rec.Code)
    count := testutil.ToFloat64(requestsTotal.WithLabelValues("GET", "/test", "200"))
    assert.Equal(t, 1.0, count)
}
```

## Custom Bucket Configuration

Tailor buckets to your API latency profile:

```go
// Fast API (< 100ms typical)
Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5}

// Standard web API (< 1s typical)
Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1.0, 5.0}

// Slower API or with external calls (< 5s typical)
Buckets: []float64{0.1, 0.5, 1.0, 5.0, 10.0, 30.0}
```
