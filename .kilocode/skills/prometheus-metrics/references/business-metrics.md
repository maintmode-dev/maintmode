# Business Metrics for MaintMode

Domain-specific metrics for tracking maintenance operations, conflict detection, and resource utilization.

## Table of Contents
- [Maintenance Lifecycle Metrics](#maintenance-lifecycle-metrics)
- [Conflict Detection Metrics](#conflict-detection-metrics)
- [Resource Utilization Metrics](#resource-utilization-metrics)
- [Custom Collector Patterns](#custom-collector-patterns)
- [Event-Based Metrics](#event-based-metrics)

## Maintenance Lifecycle Metrics

Track maintenance operations through their lifecycle:

```go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // Counter: Total maintenances created
    maintenancesCreated = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "maintenances_created_total",
            Help: "Total number of maintenances created",
        },
        []string{"status"}, // draft, approved, scheduled
    )

    // Counter: Maintenance state transitions
    maintenanceTransitions = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "maintenance_transitions_total",
            Help: "Total number of maintenance state transitions",
        },
        []string{"from_status", "to_status"},
    )

    // Gauge: Current maintenances by status
    maintenancesCurrent = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "maintenances_current",
            Help: "Current number of maintenances by status",
        },
        []string{"status"}, // draft, scheduled, in_progress, completed, cancelled
    )

    // Histogram: Maintenance duration
    maintenanceDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "maintenance_duration_seconds",
            Help:    "Duration of completed maintenances",
            Buckets: []float64{60, 300, 900, 1800, 3600, 7200, 14400}, // 1m to 4h
        },
        []string{"status"}, // completed, cancelled
    )

    // Counter: Maintenance operations
    maintenanceOperations = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "maintenance_operations_total",
            Help: "Total maintenance operations",
        },
        []string{"operation"}, // create, update, start, complete, cancel, approve
    )
)

// RecordMaintenanceCreated tracks new maintenance creation
func RecordMaintenanceCreated(status string) {
    maintenancesCreated.WithLabelValues(status).Inc()
    maintenancesCurrent.WithLabelValues(status).Inc()
    maintenanceOperations.WithLabelValues("create").Inc()
}

// RecordMaintenanceTransition tracks status changes
func RecordMaintenanceTransition(fromStatus, toStatus string) {
    maintenanceTransitions.WithLabelValues(fromStatus, toStatus).Inc()
    maintenancesCurrent.WithLabelValues(fromStatus).Dec()
    maintenancesCurrent.WithLabelValues(toStatus).Inc()
}

// RecordMaintenanceCompleted tracks completed maintenances
func RecordMaintenanceCompleted(durationSeconds float64) {
    maintenanceDuration.WithLabelValues("completed").Observe(durationSeconds)
    maintenancesCurrent.WithLabelValues("in_progress").Dec()
    maintenancesCurrent.WithLabelValues("completed").Inc()
    maintenanceOperations.WithLabelValues("complete").Inc()
}

// RecordMaintenanceCancelled tracks cancelled maintenances
func RecordMaintenanceCancelled(status string) {
    maintenancesCurrent.WithLabelValues(status).Dec()
    maintenancesCurrent.WithLabelValues("cancelled").Inc()
    maintenanceOperations.WithLabelValues("cancel").Inc()
}
```

## Conflict Detection Metrics

Track resource conflicts and resolution:

```go
package metrics

var (
    // Counter: Conflicts detected
    conflictsDetected = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "maintenance_conflicts_detected_total",
            Help: "Total number of maintenance conflicts detected",
        },
        []string{"conflict_type"}, // resource, time
    )

    // Gauge: Current active conflicts
    conflictsActive = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "maintenance_conflicts_active",
            Help: "Number of active maintenance conflicts",
        },
        []string{"conflict_type"},
    )

    // Counter: Conflicts resolved
    conflictsResolved = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "maintenance_conflicts_resolved_total",
            Help: "Total number of conflicts resolved",
        },
        []string{"resolution"}, // rescheduled, cancelled, ignored
    )

    // Histogram: Time to resolve conflicts
    conflictResolutionDuration = promauto.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "maintenance_conflict_resolution_duration_seconds",
            Help:    "Time taken to resolve conflicts",
            Buckets: []float64{60, 300, 900, 3600, 86400}, // 1m to 1d
        },
    )
)

// RecordConflictDetected tracks new conflicts
func RecordConflictDetected(conflictType string) {
    conflictsDetected.WithLabelValues(conflictType).Inc()
    conflictsActive.WithLabelValues(conflictType).Inc()
}

// RecordConflictResolved tracks conflict resolution
func RecordConflictResolved(conflictType, resolution string, durationSeconds float64) {
    conflictsResolved.WithLabelValues(resolution).Inc()
    conflictsActive.WithLabelValues(conflictType).Dec()
    conflictResolutionDuration.Observe(durationSeconds)
}
```

## Resource Utilization Metrics

Track resource allocation and usage:

```go
package metrics

var (
    // Gauge: Total resources
    resourcesTotal = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "resources_total",
            Help: "Total number of resources in system",
        },
    )

    // Gauge: Resources in use
    resourcesInUse = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "resources_in_use",
            Help: "Number of resources currently in use by maintenances",
        },
    )

    // Gauge: Resource utilization percentage
    resourceUtilization = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "resource_utilization_percent",
            Help: "Percentage of resources currently in use",
        },
    )

    // Counter: Resource assignments
    resourceAssignments = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "resource_assignments_total",
            Help: "Total number of resource assignments to maintenances",
        },
        []string{"operation"}, // assign, unassign
    )

    // Histogram: Resources per maintenance
    resourcesPerMaintenance = promauto.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "maintenance_resources_count",
            Help:    "Number of resources per maintenance",
            Buckets: []float64{1, 2, 5, 10, 20, 50},
        },
    )
)

// RecordResourceAssigned tracks resource allocation
func RecordResourceAssigned() {
    resourceAssignments.WithLabelValues("assign").Inc()
    resourcesInUse.Inc()
    updateResourceUtilization()
}

// RecordResourceUnassigned tracks resource deallocation
func RecordResourceUnassigned() {
    resourceAssignments.WithLabelValues("unassign").Inc()
    resourcesInUse.Dec()
    updateResourceUtilization()
}

// UpdateResourceUtilization calculates utilization percentage
func updateResourceUtilization() {
    total := testutil.ToFloat64(resourcesTotal)
    inUse := testutil.ToFloat64(resourcesInUse)

    if total > 0 {
        utilization := (inUse / total) * 100
        resourceUtilization.Set(utilization)
    }
}
```

## Custom Collector Patterns

Collect business metrics from database:

```go
package metrics

import (
    "context"
    "time"

    "github.com/prometheus/client_golang/prometheus"

    "github.com/ruko1202/maintmode/internal/storages/maintenances"
)

type maintenanceCollector struct {
    store *maintenances.Store

    totalDesc              *prometheus.Desc
    byStatusDesc           *prometheus.Desc
    upcomingDesc           *prometheus.Desc
    averageDurationDesc    *prometheus.Desc
}

func NewMaintenanceCollector(store *maintenances.Store) prometheus.Collector {
    return &maintenanceCollector{
        store: store,
        totalDesc: prometheus.NewDesc(
            "maintenances_total",
            "Total number of maintenances",
            nil, nil,
        ),
        byStatusDesc: prometheus.NewDesc(
            "maintenances_by_status",
            "Number of maintenances by status",
            []string{"status"}, nil,
        ),
        upcomingDesc: prometheus.NewDesc(
            "maintenances_upcoming",
            "Number of upcoming maintenances in next N days",
            []string{"days"}, nil,
        ),
        averageDurationDesc: prometheus.NewDesc(
            "maintenance_average_duration_seconds",
            "Average duration of completed maintenances",
            nil, nil,
        ),
    }
}

func (c *maintenanceCollector) Describe(ch chan<- *prometheus.Desc) {
    ch <- c.totalDesc
    ch <- c.byStatusDesc
    ch <- c.upcomingDesc
    ch <- c.averageDurationDesc
}

func (c *maintenanceCollector) Collect(ch chan<- prometheus.Metric) {
    ctx := context.Background()

    // Get statistics from store
    stats, err := c.store.GetStatistics(ctx)
    if err != nil {
        // Log error but don't fail collection
        return
    }

    // Total maintenances
    ch <- prometheus.MustNewConstMetric(
        c.totalDesc,
        prometheus.GaugeValue,
        float64(stats.Total),
    )

    // By status
    for status, count := range stats.ByStatus {
        ch <- prometheus.MustNewConstMetric(
            c.byStatusDesc,
            prometheus.GaugeValue,
            float64(count),
            status,
        )
    }

    // Upcoming maintenances
    ch <- prometheus.MustNewConstMetric(
        c.upcomingDesc,
        prometheus.GaugeValue,
        float64(stats.Upcoming7Days),
        "7",
    )
    ch <- prometheus.MustNewConstMetric(
        c.upcomingDesc,
        prometheus.GaugeValue,
        float64(stats.Upcoming30Days),
        "30",
    )

    // Average duration
    if stats.AverageDuration > 0 {
        ch <- prometheus.MustNewConstMetric(
            c.averageDurationDesc,
            prometheus.GaugeValue,
            stats.AverageDuration.Seconds(),
        )
    }
}

// Register collector
func RegisterMaintenanceCollector(store *maintenances.Store) {
    prometheus.MustRegister(NewMaintenanceCollector(store))
}
```

## Event-Based Metrics

Instrument service layer methods:

```go
package maint

import (
    "context"
    "time"

    "github.com/ruko1202/maintmode/internal/entity"
    "github.com/ruko1202/maintmode/internal/metrics"
)

type Service struct {
    store *maintenances.Store
}

// CreateDraft with metrics
func (s *Service) CreateDraft(ctx context.Context, cmd entity.CreateDraftCommand) (*entity.Maintenance, error) {
    start := time.Now()

    maint, err := s.store.Create(ctx, cmd)

    if err != nil {
        metrics.RecordMaintenanceOperation("create", "error")
        return nil, err
    }

    // Record successful creation
    metrics.RecordMaintenanceCreated("draft")
    metrics.RecordMaintenanceOperation("create", "success")

    // Record resources
    metrics.RecordResourcesPerMaintenance(float64(len(cmd.Resources)))

    return maint, nil
}

// Start maintenance with metrics
func (s *Service) Start(ctx context.Context, id uuid.UUID) error {
    start := time.Now()

    // Get current maintenance
    maint, err := s.store.Get(ctx, id)
    if err != nil {
        metrics.RecordMaintenanceOperation("start", "error")
        return err
    }

    // Update status
    err = s.store.UpdateStatus(ctx, id, entity.StatusInProgress)
    if err != nil {
        metrics.RecordMaintenanceOperation("start", "error")
        return err
    }

    // Record transition
    metrics.RecordMaintenanceTransition(string(maint.Status), string(entity.StatusInProgress))
    metrics.RecordMaintenanceOperation("start", "success")

    return nil
}

// Complete maintenance with metrics
func (s *Service) Complete(ctx context.Context, id uuid.UUID) error {
    maint, err := s.store.Get(ctx, id)
    if err != nil {
        metrics.RecordMaintenanceOperation("complete", "error")
        return err
    }

    err = s.store.UpdateStatus(ctx, id, entity.StatusCompleted)
    if err != nil {
        metrics.RecordMaintenanceOperation("complete", "error")
        return err
    }

    // Calculate duration
    duration := time.Since(maint.StartedAt).Seconds()

    // Record completion
    metrics.RecordMaintenanceCompleted(duration)
    metrics.RecordMaintenanceOperation("complete", "success")

    return nil
}
```

## Service-Level Business Metrics

Track business operations at service layer:

```go
package metrics

var (
    // Business operation outcomes
    businessOperations = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "business_operations_total",
            Help: "Total business operations",
        },
        []string{"operation", "outcome"}, // success, error, validation_failed
    )

    // Business operation duration
    businessOperationDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "business_operation_duration_seconds",
            Help:    "Business operation duration",
            Buckets: []float64{0.01, 0.1, 0.5, 1.0, 5.0},
        },
        []string{"operation"},
    )
)

// RecordMaintenanceOperation tracks operation outcomes
func RecordMaintenanceOperation(operation, outcome string) {
    businessOperations.WithLabelValues(operation, outcome).Inc()
}

// RecordOperationDuration tracks operation timing
func RecordOperationDuration(operation string, duration float64) {
    businessOperationDuration.WithLabelValues(operation).Observe(duration)
}
```

## Integration Example: Conflict Service

```go
package conflicts

import (
    "context"

    "github.com/ruko1202/maintmode/internal/entity"
    "github.com/ruko1202/maintmode/internal/metrics"
)

type Service struct {
    store *conflicts.Store
}

// GetConflicts with metrics
func (s *Service) GetConflicts(ctx context.Context, maintID uuid.UUID) ([]entity.Conflict, error) {
    start := time.Now()

    conflicts, err := s.store.GetConflicts(ctx, maintID)
    if err != nil {
        metrics.RecordBusinessOperation("get_conflicts", "error")
        return nil, err
    }

    // Record metrics
    metrics.RecordBusinessOperation("get_conflicts", "success")
    metrics.RecordOperationDuration("get_conflicts", time.Since(start).Seconds())

    // Record conflicts found
    if len(conflicts) > 0 {
        for _, conflict := range conflicts {
            metrics.RecordConflictDetected(string(conflict.Type))
        }
    }

    return conflicts, nil
}
```

## Best Practices for Business Metrics

1. **Track domain events** - maintenance created, started, completed
2. **Record state transitions** - draft → scheduled → in_progress → completed
3. **Measure business durations** - maintenance execution time, conflict resolution time
4. **Use custom collectors** for aggregated statistics from database
5. **Coordinate with service layer** - instrument business logic, not just infrastructure
6. **Track error types** - validation errors, business rule violations, system errors
7. **Monitor resource utilization** - track allocation, capacity, contention
8. **Use appropriate metric types**:
   - Counter: events (created, cancelled)
   - Gauge: current state (active maintenances)
   - Histogram: durations (execution time)
9. **Avoid high cardinality** - use status, type, not IDs
10. **Test business metrics** - ensure they're recorded correctly in service tests
