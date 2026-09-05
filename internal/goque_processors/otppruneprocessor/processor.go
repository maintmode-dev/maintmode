// Package otppruneprocessor handles otp.prune goque tasks. The task is produced
// once per cron tick by a periodic job and carries the retention tunables
// (window + batch limit) in its payload; at process time it deletes one-time
// codes whose expires_at is older than the retention window, in bounded batches.
package otppruneprocessor

import (
	"cmp"
	"context"
	"time"

	"github.com/ruko1202/goque"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

const (
	// Both defaults must match the ones the service falls back to and the values
	// shipped in deployment config, so an unset payload field behaves the same
	// wherever it is resolved. 24h is short on purpose: the row holds a code
	// digest and a session nonce, and the code itself lives only minutes.
	defaultRetention  = 24 * time.Hour
	defaultBatchLimit = 1000
)

// Pruner deletes one-time codes older than the retention window. Defined
// consumer-side so the processor can be tested with a mock.
type Pruner interface {
	Prune(ctx context.Context, retention time.Duration, batchLimit int64) error
}

// NewTaskProcessor returns the goque TaskProcessor for ProcessorTaskOTPPrune. It
// reads the retention window and batch limit from the task payload and runs the
// prune.
func NewTaskProcessor(pruner Pruner) goque.TaskProcessor {
	return goque.NewTypedTaskProcessor(
		goque.TypedTaskProcessorFunc[entity.ProcessorTaskPayloadOTPPrune](
			func(ctx context.Context, task *goque.TypedTask[entity.ProcessorTaskPayloadOTPPrune]) error {
				ctx, span := xlog.WithOperationSpan(ctx, "service.OTP.PruneProcessor.ProcessTask")
				defer span.End()

				return pruner.Prune(ctx, task.Payload.Retention, task.Payload.BatchLimit)
			},
		),
		goque.WithCancelTaskWhenPayloadDecodeError[entity.ProcessorTaskPayloadOTPPrune](),
	)
}

// NewTaskFactory returns the goque PeriodicJobFactory that produces one otp.prune
// task per cron tick, stamping the configured retention window and batch limit
// into the payload.
//
// The external id is bucketed to the day so that, with several replicas all
// ticking the same schedule, only the first insert for a given day succeeds; the
// others collide on the (type, external_id) unique key and return
// goque.ErrDuplicateTask -- the desired at-most-once-per-day fan-in. On a single
// replica this never fires.
func NewTaskFactory(retention time.Duration, batchLimit int64) goque.PeriodicJobFactory {
	return func(_ context.Context) (*goque.Task, error) {
		return goque.NewTaskWithPayloadAndExternalID(
			entity.ProcessorTaskOTPPrune,
			entity.ProcessorTaskPayloadOTPPrune{
				Retention:  cmp.Or(retention, defaultRetention),
				BatchLimit: cmp.Or(batchLimit, defaultBatchLimit),
			},
			otpPruneExternalID(xtime.UTCNow()),
		)
	}
}
