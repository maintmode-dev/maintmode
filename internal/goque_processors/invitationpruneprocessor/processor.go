// Package invitationpruneprocessor handles invitation.prune goque tasks. The
// task is produced once per cron tick by a periodic job and carries the
// retention tunables (window + batch limit) in its payload; at process time it
// deletes terminal invitations older than the retention window in bounded
// batches.
package invitationpruneprocessor

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
	defaultRetention  = 365 * 24 * time.Hour
	defaultBatchLimit = 1000
)

// Pruner deletes terminal invitations older than the retention window. Defined
// consumer-side so the processor can be tested with a mock.
type Pruner interface {
	Prune(ctx context.Context, retention time.Duration, batchLimit int64) error
}

// NewTaskProcessor returns the goque TaskProcessor for
// ProcessorTaskInvitationPrune. It reads the retention window and batch limit
// from the task payload and runs the prune.
func NewTaskProcessor(pruner Pruner) goque.TaskProcessor {
	return goque.NewTypedTaskProcessor(
		goque.TypedTaskProcessorFunc[entity.ProcessorTaskPayloadInvitationPrune](
			func(ctx context.Context, task *goque.TypedTask[entity.ProcessorTaskPayloadInvitationPrune]) error {
				ctx, span := xlog.WithOperationSpan(ctx, "service.Invitation.PruneProcessor.ProcessTask")
				defer span.End()

				return pruner.Prune(ctx, task.Payload.Retention, task.Payload.BatchLimit)
			},
		),
		goque.WithCancelTaskWhenPayloadDecodeError[entity.ProcessorTaskPayloadInvitationPrune](),
	)
}

// NewTaskFactory returns the goque PeriodicJobFactory that produces one
// invitation.prune task per cron tick, stamping the configured retention window
// and batch limit into the payload.
//
// The external id is bucketed to the day so that, with several replicas all
// ticking the same schedule, only the first insert for a given day succeeds; the
// others collide on the (type, external_id) unique key and return
// goque.ErrDuplicateTask — the desired at-most-once-per-day fan-in. On a single
// replica this never fires.
func NewTaskFactory(retention time.Duration, batchLimit int64) goque.PeriodicJobFactory {
	return func(_ context.Context) (*goque.Task, error) {
		return goque.NewTaskWithPayloadAndExternalID(
			entity.ProcessorTaskInvitationPrune,
			entity.ProcessorTaskPayloadInvitationPrune{
				Retention:  cmp.Or(retention, defaultRetention),
				BatchLimit: cmp.Or(batchLimit, defaultBatchLimit),
			},
			invitationPruneExternalID(xtime.UTCNow()),
		)
	}
}
