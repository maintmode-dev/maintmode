// Package autocancelprocessor handles maint.auto.cancel goque tasks. The task is
// produced once per cron tick by a periodic job and carries the sweep tunables
// (grace threshold + batch limit) in its payload; at process time it cancels
// not-started maintenances (draft or planned) whose planned start passed the
// grace window.
package autocancelprocessor

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
	defaultThreshold = 15 * time.Minute
	defaultLimit     = 100
)

// UnStartedCanceller cancels not-started maintenances (draft or planned) whose
// planned start is before cutoff. Defined consumer-side so the processor can be
// tested with a mock.
type UnStartedCanceller interface {
	CancelUnStarted(ctx context.Context, cutoff time.Time, limit int64) error
}

// NewTaskProcessor returns the goque TaskProcessor for ProcessorTaskMaintAutoCancel.
// It reads the grace threshold and batch limit from the task payload, derives the
// cutoff (now - threshold) at process time, and runs the sweep.
func NewTaskProcessor(canceller UnStartedCanceller) goque.TaskProcessor {
	return goque.NewTypedTaskProcessor(
		goque.TypedTaskProcessorFunc[entity.ProcessorTaskPayloadMaintAutoCancel](
			func(ctx context.Context, task *goque.TypedTask[entity.ProcessorTaskPayloadMaintAutoCancel]) error {
				ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.AutoCancelProcessor.ProcessTask")
				defer span.End()

				cutoff := xtime.UTCNow().Add(-task.Payload.Threshold)
				limit := task.Payload.Limit

				return canceller.CancelUnStarted(ctx, cutoff, limit)
			},
		),
		goque.WithCancelTaskWhenPayloadDecodeError[entity.ProcessorTaskPayloadMaintAutoCancel](),
	)
}

// NewTaskFactory returns the goque PeriodicJobFactory that produces one
// maint.auto.cancel task per cron tick, stamping the configured grace threshold
// and batch limit into the payload.
//
// The external id is bucketed to the minute so that, with several replicas all
// ticking the same schedule, only the first insert for a given minute succeeds;
// the others collide on the (type, external_id) unique key and return
// goque.ErrDuplicateTask — the desired at-most-once-per-minute fan-in. The losing
// inserts are correctness-neutral but goque v0.8.9 logs them at ERROR and does not
// retry (see the registration note in bootstrap.NewTaskProcessors); on a single
// replica this never fires.
func NewTaskFactory(threshold time.Duration, limit int64) goque.PeriodicJobFactory {
	return func(_ context.Context) (*goque.Task, error) {
		return goque.NewTaskWithPayloadAndExternalID(
			entity.ProcessorTaskMaintAutoCancel,
			entity.ProcessorTaskPayloadMaintAutoCancel{
				Threshold: cmp.Or(threshold, defaultThreshold),
				Limit:     cmp.Or(limit, defaultLimit),
			},
			autoCancelExternalID(xtime.UTCNow()),
		)
	}
}
