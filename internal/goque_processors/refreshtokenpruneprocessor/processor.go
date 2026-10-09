// Package refreshtokenpruneprocessor handles refresh_token.prune goque tasks.
// The task is produced once per cron tick by a periodic job and carries the
// retention tunables (window + batch limit) in its payload; at process time it
// deletes refresh-token rows whose expires_at is older than the retention
// window, in bounded batches.
package refreshtokenpruneprocessor

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
	// DefaultCronSpec is the schedule used when config leaves it empty: daily
	// at 03:30 UTC, after the other retention sweeps.
	DefaultCronSpec = "30 3 * * *"

	// Both defaults must match the ones the token service falls back to and the
	// values shipped in deployment config, so an unset field behaves the same
	// wherever it is resolved.
	defaultRetention  = 24 * time.Hour
	defaultBatchLimit = 1000
)

// Pruner deletes refresh-token rows past the retention window. Defined
// consumer-side so the processor can be tested with a mock.
type Pruner interface {
	PruneRefreshTokens(ctx context.Context, retention time.Duration, batchLimit int64) error
}

// NewTaskProcessor returns the goque TaskProcessor for
// ProcessorTaskRefreshTokenPrune. It reads the retention window and batch limit
// from the task payload and runs the prune.
func NewTaskProcessor(pruner Pruner) goque.TaskProcessor {
	return goque.NewTypedTaskProcessor(
		goque.TypedTaskProcessorFunc[entity.ProcessorTaskPayloadRefreshTokenPrune](
			func(ctx context.Context, task *goque.TypedTask[entity.ProcessorTaskPayloadRefreshTokenPrune]) error {
				ctx, span := xlog.WithOperationSpan(ctx, "service.Token.PruneProcessor.ProcessTask")
				defer span.End()

				return pruner.PruneRefreshTokens(ctx, task.Payload.Retention, task.Payload.BatchLimit)
			},
		),
		goque.WithCancelTaskWhenPayloadDecodeError[entity.ProcessorTaskPayloadRefreshTokenPrune](),
	)
}

// NewTaskFactory returns the goque PeriodicJobFactory that produces one
// refresh_token.prune task per cron tick, stamping the configured retention
// window and batch limit -- or their defaults when unset -- into the payload.
//
// The external id is bucketed to the day so that, with several replicas all
// ticking the same schedule, only the first insert for a given day succeeds.
func NewTaskFactory(retention time.Duration, batchLimit int64) goque.PeriodicJobFactory {
	return func(_ context.Context) (*goque.Task, error) {
		return goque.NewTaskWithPayloadAndExternalID(
			entity.ProcessorTaskRefreshTokenPrune,
			entity.ProcessorTaskPayloadRefreshTokenPrune{
				Retention:  cmp.Or(retention, defaultRetention),
				BatchLimit: cmp.Or(batchLimit, defaultBatchLimit),
			},
			refreshTokenPruneExternalID(xtime.UTCNow()),
		)
	}
}
