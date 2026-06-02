package scheduler

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/goque"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Cancel moves a pending task to status=canceled. No-op if the task is already
// terminal, so it is safe to call best-effort. Like ScheduleDelayed it honors a
// maintmode tx on ctx (cancel participates in the caller's tx).
func (s *Service) Cancel(ctx context.Context, taskID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Scheduler.Cancel")
	defer span.End()

	if tx, ok := dbtx.TxFromContext(ctx); ok {
		ctx = goque.WithTx(ctx, tx)
	}

	return s.queue.CancelTask(ctx, taskID)
}
