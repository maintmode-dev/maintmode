package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ruko1202/goque"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// ScheduleDelayed enqueues a typed task of taskType to be processed after delay
// (NextAttemptAt = now+delay), returning the goque task id so the caller can
// cancel it later. If a maintmode tx is attached to ctx, the insert joins it
// (transactional outbox), making the enqueue atomic with the caller's writes.
func (s *Service) ScheduleDelayed(
	ctx context.Context,
	taskType string,
	payload any,
	delay time.Duration,
	idempotencyKey string,
) (uuid.UUID, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Scheduler.ScheduleDelayed",
		xfield.String("taskType", taskType),
	)
	defer span.End()

	if tx, ok := dbtx.TxFromContext(ctx); ok {
		ctx = goque.WithTx(ctx, tx)
	}

	task, err := goque.NewTaskWithPayloadAndExternalID(taskType, payload, idempotencyKey)
	if err != nil {
		return uuid.Nil, fmt.Errorf("build task: %w", err)
	}

	if delay > 0 {
		task.NextAttemptAt = task.NextAttemptAt.Add(delay)
	}

	if err := s.queue.AddTaskToQueue(ctx, task); err != nil {
		xlog.Error(ctx, "schedule task failed", xfield.Error(err))
		return uuid.Nil, err
	}

	return task.ID, nil
}
