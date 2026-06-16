package auditprocessor

import (
	"context"
	"fmt"

	"github.com/ruko1202/goque"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/metrics"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

func (p *processor) ProcessTask(ctx context.Context, task *goque.TypedTask[entity.ProcessorTaskPayloadAuditWrite]) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Audit.WriteProcessor.ProcessTask",
		xfield.String("event_id", task.Payload.EventID.String()),
		xfield.String("action", string(task.Payload.Action)),
	)
	defer span.End()

	// Backstop for the no-listener-in-tx contract (review finding H3). The
	// always-outbox design already guarantees this runs after commit, but assert
	// it loudly so the invariant can't silently regress if the drain ever runs
	// inside a tx. Mirrors the sender.Send guard.
	if _, ok := dbtx.TxFromContext(ctx); ok {
		return fmt.Errorf("audit write processor must not run inside a DB tx")
	}

	if err := p.auditor.AddLog(ctx, task.Payload.ToAuditEntry()); err != nil {
		// Return the error so goque retries. The idempotent INSERT
		// (ON CONFLICT event_id DO NOTHING) makes a retry safe after a partial
		// write. The counter makes a stuck audit trail visible (alerted on).
		metrics.AuditWriteError(ctx)
		xlog.Error(ctx, "failed to write audit log", xfield.Error(err))
		return fmt.Errorf("audit write processor: add log: %w", err)
	}

	return nil
}
