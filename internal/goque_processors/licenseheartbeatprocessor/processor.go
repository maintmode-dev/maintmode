// Package licenseheartbeatprocessor handles license.heartbeat goque tasks
// (SaaS mode only). The task is produced once per cron tick with an
// empty payload; at process time the license service collects a fresh
// seat/activity report, sends the heartbeat to Console and caches the returned
// license. Console-side failures are fail-open inside the service (warn + keep
// the cached license), so a processing error here means a local failure and
// the next tick is the retry.
package licenseheartbeatprocessor

import (
	"context"

	"github.com/ruko1202/goque"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// Refresher performs one heartbeat tick. Defined consumer-side so the
// processor can be tested with a mock.
type Refresher interface {
	Refresh(ctx context.Context) error
}

// emptyPayload pins the payload shape: the report is collected at process
// time, never snapshotted at enqueue time (a stale seat count must not reach
// Console).
type emptyPayload struct{}

// NewTaskProcessor returns the goque TaskProcessor for
// ProcessorTaskLicenseHeartbeat.
func NewTaskProcessor(refresher Refresher) goque.TaskProcessor {
	return goque.NewTypedTaskProcessor(
		goque.TypedTaskProcessorFunc[emptyPayload](
			func(ctx context.Context, _ *goque.TypedTask[emptyPayload]) error {
				ctx, span := xlog.WithOperationSpan(ctx, "service.License.HeartbeatProcessor.ProcessTask")
				defer span.End()

				return refresher.Refresh(ctx)
			},
		),
		goque.WithCancelTaskWhenPayloadDecodeError[emptyPayload](),
	)
}

// NewTaskFactory returns the goque PeriodicJobFactory that produces one
// license.heartbeat task per cron tick.
//
// The external id is bucketed to the minute (the heartbeat cadence): with
// several replicas ticking the same schedule only the first insert per minute
// succeeds; the rest collide on the (type, external_id) unique key — the same
// at-most-once fan-in as maint.auto.cancel (see the goque v0.8.9 duplicate-key
// ERROR note in bootstrap.NewTaskProcessors).
func NewTaskFactory() goque.PeriodicJobFactory {
	return func(_ context.Context) (*goque.Task, error) {
		return goque.NewTaskWithPayloadAndExternalID(
			entity.ProcessorTaskLicenseHeartbeat,
			emptyPayload{},
			licenseHeartbeatExternalID(xtime.UTCNow()),
		)
	}
}
