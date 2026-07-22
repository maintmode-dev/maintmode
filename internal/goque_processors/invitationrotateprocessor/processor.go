// Package invitationrotateprocessor handles invitation.rotate goque tasks. The
// task is produced once per cron tick by a periodic job and carries the batch
// limit in its payload; at process time it flips pending invitations whose
// expires_at has passed to the persisted 'expired' status in bounded batches.
package invitationrotateprocessor

import (
	"cmp"
	"context"

	"github.com/ruko1202/goque"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

const defaultBatchLimit = 1000

// Rotater flips expired-pending invitations to the persisted 'expired' status.
// Defined consumer-side so the processor can be tested with a mock.
type Rotater interface {
	Rotate(ctx context.Context, batchLimit int64) error
}

// NewTaskProcessor returns the goque TaskProcessor for
// ProcessorTaskInvitationRotate. It reads the batch limit from the task payload
// and runs the rotation.
func NewTaskProcessor(rotater Rotater) goque.TaskProcessor {
	return goque.NewTypedTaskProcessor(
		goque.TypedTaskProcessorFunc[entity.ProcessorTaskPayloadInvitationRotate](
			func(ctx context.Context, task *goque.TypedTask[entity.ProcessorTaskPayloadInvitationRotate]) error {
				ctx, span := xlog.WithOperationSpan(ctx, "service.Invitation.RotateProcessor.ProcessTask")
				defer span.End()

				return rotater.Rotate(ctx, task.Payload.BatchLimit)
			},
		),
		goque.WithCancelTaskWhenPayloadDecodeError[entity.ProcessorTaskPayloadInvitationRotate](),
	)
}

// NewTaskFactory returns the goque PeriodicJobFactory that produces one
// invitation.rotate task per cron tick, stamping the configured batch limit into
// the payload.
//
// The external id is bucketed to the day so that, with several replicas all
// ticking the same schedule, only the first insert for a given day succeeds; the
// others collide on the (type, external_id) unique key and return
// goque.ErrDuplicateTask — the desired at-most-once-per-day fan-in. On a single
// replica this never fires.
func NewTaskFactory(batchLimit int64) goque.PeriodicJobFactory {
	return func(_ context.Context) (*goque.Task, error) {
		return goque.NewTaskWithPayloadAndExternalID(
			entity.ProcessorTaskInvitationRotate,
			entity.ProcessorTaskPayloadInvitationRotate{
				BatchLimit: cmp.Or(batchLimit, defaultBatchLimit),
			},
			invitationRotateExternalID(xtime.UTCNow()),
		)
	}
}
