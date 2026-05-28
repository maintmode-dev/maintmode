// Package metrics holds application-level OpenTelemetry instruments.
//
// Instruments are registered once against the global meter provider
// (set up in config.InitMetricExporter) and exposed as plain helper
// functions, so call sites just record an event —
// metrics.RateLimiterFallback(ctx) — without holding a counter or
// importing the OTel SDK. Recording is a cross-cutting concern, not
// object state.
package metrics

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
)

const meterName = "maintmode"

var meter = otel.Meter(meterName)

// rateLimiterFallback counts each time the rate limiter falls back to
// its per-replica in-memory store because Redis was unreachable. The
// Prometheus alert RateLimiterRedisFallback watches it.
var rateLimiterFallback = mustInt64Counter(
	"ratelimit_redis_failures_total",
	"Rate limiter Redis errors that triggered the in-memory fallback.",
)

// RateLimiterFallback records one rate-limiter Redis fallback event.
func RateLimiterFallback(ctx context.Context) {
	rateLimiterFallback.Add(ctx, 1)
}

// mustInt64Counter registers a counter or returns a no-op one. The
// instrument name is a compile-time constant, so the only error path
// is a programmer typo; degrading to no-op keeps startup safe.
func mustInt64Counter(name, desc string) metric.Int64Counter {
	c, err := meter.Int64Counter(name, metric.WithDescription(desc))
	if err != nil {
		noop, _ := metricnoop.NewMeterProvider().Meter(meterName).Int64Counter(name)
		return noop
	}
	return c
}
