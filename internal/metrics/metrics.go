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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
)

const meterName = "maintmode"

var meter = otel.Meter(meterName)

// rateLimiterFallback counts each time the rate limiter falls back to
// its per-replica in-memory store because Valkey was unreachable. The
// Prometheus alert RateLimiterValkeyFallback watches it.
var rateLimiterFallback = mustInt64Counter(
	"ratelimit_valkey_failures_total",
	"Rate limiter Valkey errors that triggered the in-memory fallback.",
)

// RateLimiterFallback records one rate-limiter Valkey fallback event.
func RateLimiterFallback(ctx context.Context) {
	rateLimiterFallback.Add(ctx, 1)
}

// maintNotifyDispatchErrors counts maintenance-notification dispatch
// failures. The dispatch path swallows these so a business operation
// that already committed still reports success; each increment is a
// lifecycle notification that was silently dropped. Alert on rate>0.
// The "reason" label distinguishes the failure stage (see
// MaintNotifyDispatchError* helpers).
var maintNotifyDispatchErrors = mustInt64Counter(
	"maint_notify_dispatch_errors_total",
	"Maintenance notification dispatch failures (notification dropped), labeled by reason.",
)

const (
	dispatchReasonResolve = "resolve"
	dispatchReasonRender  = "render"
)

// MaintNotifyDispatchResolveError records a dispatch drop caused by
// failing to resolve a maintenance's notify targets (targets DB
// unreachable).
func MaintNotifyDispatchResolveError(ctx context.Context) {
	maintNotifyDispatchErrors.Add(ctx, 1, metric.WithAttributes(
		attribute.String("reason", dispatchReasonResolve),
	))
}

// MaintNotifyDispatchRenderError records a dispatch drop caused by a
// template render failure.
func MaintNotifyDispatchRenderError(ctx context.Context) {
	maintNotifyDispatchErrors.Add(ctx, 1, metric.WithAttributes(
		attribute.String("reason", dispatchReasonRender),
	))
}

// maintNotifyOwnerMentionDegraded counts notifications delivered without the
// owner's messenger handle when one was expected. The notification still goes
// out — the mention is a decoration, never a delivery condition — so the
// degradation is invisible in logs, which already carry unrelated usersummary
// noise from read paths.
//
// An empty handle is deliberately NOT counted here: it is the feature's
// switched-off state, true for every user who has not filled one in, and
// counting it would make a rate>0 alert permanently red until someone silenced
// it. Adoption, if it ever needs measuring, is a separate metric.
var maintNotifyOwnerMentionDegraded = mustInt64Counter(
	"maint_notify_owner_mention_degraded_total",
	"Notifications sent with the owner's display name instead of a messenger handle, labeled by reason.",
)

// OwnerMentionReason labels a degraded owner mention. The type keeps call sites
// from inventing new label values, which would fragment the metric.
type OwnerMentionReason string

const (
	// OwnerMentionRejected: the stored handle failed the render-time sanitizer
	// (control characters or Slack markup metacharacters). A value that got
	// past input validation is in the database — investigate how.
	OwnerMentionRejected OwnerMentionReason = "rejected"
	// OwnerMentionUnresolved: the owner's user id could not be resolved at all,
	// so the message names "Unknown user". Usually the auth service being
	// unavailable to a background reminder task.
	OwnerMentionUnresolved OwnerMentionReason = "unresolved"
)

// MaintNotifyOwnerMentionDegraded records one notification that lost its owner
// mention.
func MaintNotifyOwnerMentionDegraded(ctx context.Context, reason OwnerMentionReason) {
	maintNotifyOwnerMentionDegraded.Add(ctx, 1, metric.WithAttributes(
		attribute.String("reason", string(reason)),
	))
}

// auditWriteErrors counts audit_log write failures in the audit-write goque
// processor (RUK-179). The processor returns the error so goque retries, but a
// sustained rate>0 means the audit trail is falling behind or a write is
// permanently failing — for a compliance log that must be visible. Alert on
// rate>0.
var auditWriteErrors = mustInt64Counter(
	"audit_write_errors_total",
	"Audit log write failures in the audit-write processor (task will be retried).",
)

// AuditWriteError records one failed audit_log write.
func AuditWriteError(ctx context.Context) {
	auditWriteErrors.Add(ctx, 1)
}

// Note on M4 (goque payload-decode-cancel visibility): goque already exports
// goque_payload_decode_errors_total (a Prometheus CounterVec labeled by
// task_type) on the same /metrics endpoint, so no app-level counter is needed.
// The remaining gap — an alert — is closed by the GoquePayloadDecodeErrors rule
// in monitoring/config/alerts.yml.

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
