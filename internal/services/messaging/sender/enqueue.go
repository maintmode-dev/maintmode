package messagesender

import (
	"context"
	"fmt"

	"github.com/ruko1202/goque"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"

	// dbtx is the only domain dep — used as a bridge between maintmode's
	// tx-carrying ctx and goque.WithTx. Pure infra plumbing.
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

func (s *Service) enqueue(
	ctx context.Context,
	tn entity.NotifyTransport,
	target string,
	msg entity.NotifyMessage,
	enqOpts ...EnqueueOption,
) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Messaging.enqueue",
		xfield.String("transport", string(tn)),
		xfield.String("target", target),
	)
	defer span.End()

	opts := &enqueueOpts{
		idempotencyKey: xuuid.NewString(),
		delay:          0,
	}
	for _, opt := range enqOpts {
		opt(opts)
	}

	task, err := goque.NewTaskWithPayloadAndExternalID(
		entity.ProcessorTaskMessagingSend,
		entity.ProcessorTaskPayloadEventNotify{
			TransportName: tn,
			Target:        target,
			Subject:       msg.Subject,
			Body:          msg.Body,
		},
		opts.idempotencyKey,
	)
	if err != nil {
		return fmt.Errorf("build task: %w", err)
	}

	task.NextAttemptAt = task.NextAttemptAt.Add(opts.delay)

	// If maintmode tx is attached to ctx, hand it to goque so AddTaskToQueue
	// participates in the caller's tx (transactional outbox).
	if tx, ok := dbtx.TxFromContext(ctx); ok {
		ctx = goque.WithTx(ctx, tx)
	}

	err = s.queue.AddTaskToQueue(ctx, task)
	if err != nil {
		xlog.Error(ctx, "messaging enqueue failed", xfield.Error(err))
		return err
	}

	return nil
}
