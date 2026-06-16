// Package auditpublisher publishes audited actions to the durable audit outbox.
//
// It is the publish side of the audit trail (RUK-179): a service that performs
// an audited action calls Publish, which renders the point-in-time AuditEntry
// snapshot and enqueues an audit.write goque task. The task is drained later by
// the audit-write processor — after commit, outside any tx — so a publish can
// never hold a business transaction (closes review finding H3 by construction).
//
// This is deliberately explicit, not an event bus: there is exactly one consumer
// (audit), so the path action → snapshot → outbox reads straight and the
// task-type ↔ processor link is direct. The publisher holds no audit store — it
// only enqueues — so it can run on any binary (auth publishes user/auth actions;
// maintmode will publish maint actions, RUK-182), while the audit-write processor
// that drains audit.write lives on the auth binary, which owns the audit store.
package auditpublisher

import (
	"context"
	"fmt"

	"github.com/ruko1202/goque"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// taskEnqueuer is the slice of goque.TaskQueueManager the publisher needs:
// synchronous, tx-honoring enqueue (outbox semantics). Defined consumer-side so
// the publisher depends only on the enqueue capability — *goque.TaskQueueManager
// satisfies it — not on the messaging scheduler service.
type taskEnqueuer interface {
	AddTaskToQueue(ctx context.Context, task *goque.Task) error
}

// renderer maps an audited action to the audit-write payload snapshot. Kept as
// an interface so the publisher can be unit-tested with a fake.
type renderer interface {
	Render(action audit.Action) (entity.ProcessorTaskPayloadAuditWrite, error)
}

// Publisher enqueues audit.write tasks for audited actions.
type Publisher struct {
	queue    taskEnqueuer
	renderer renderer
}

// New builds a Publisher with the production audit renderer.
func New(queue taskEnqueuer) *Publisher {
	return &Publisher{
		queue:    queue,
		renderer: audit.NewRenderer(),
	}
}

// Publish renders action to its audit snapshot and enqueues an audit.write task.
// If a maintmode tx is attached to ctx the insert joins it (transactional
// outbox), making the publish atomic with the caller's mutation; otherwise it is
// a durable autocommit enqueue. (Today's callers publish after their business tx
// commits, so they take the autocommit path; the tx-join is there for a future
// caller that publishes inside its mutation tx.)
//
// The per-event EventID is the goque external_id: a duplicated enqueue is
// rejected by goque's unique (type, external_id) index and surfaces as an error
// — not silently swallowed. The end-to-end idempotency that makes a redelivered
// task safe lives at the write layer (audit_log.event_id UNIQUE + ON CONFLICT
// DO NOTHING), not here.
func (p *Publisher) Publish(ctx context.Context, action audit.Action) error {
	payload, err := p.renderer.Render(action)
	if err != nil {
		return fmt.Errorf("render audit action: %w", err)
	}

	ctx, span := xlog.WithOperationSpan(ctx, "service.AuditPublisher.Publish",
		xfield.String("action", string(payload.Action)))
	defer span.End()

	if tx, ok := dbtx.TxFromContext(ctx); ok {
		ctx = goque.WithTx(ctx, tx)
	}

	task, err := goque.NewTaskWithPayloadAndExternalID(
		entity.ProcessorTaskAuditWrite, payload, payload.EventID.String(),
	)
	if err != nil {
		return fmt.Errorf("build audit task: %w", err)
	}

	if err := p.queue.AddTaskToQueue(ctx, task); err != nil {
		return fmt.Errorf("enqueue audit task: %w", err)
	}

	return nil
}
