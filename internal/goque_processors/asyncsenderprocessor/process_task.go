package asyncsenderprocessor

import (
	"context"

	"github.com/ruko1202/goque"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (p *queueProcessor) ProcessTask(ctx context.Context, task *goque.TypedTask[entity.ProcessorTaskPayloadEventNotify]) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Messaging.QueueProcessor.ProcessTask",
		xfield.String("transport", string(task.Payload.TransportName)),
		xfield.String("target", task.Payload.Target),
	)
	defer span.End()

	payload := task.Payload

	// The thread root comes from the payload: whoever enqueued the task decided
	// whether this message threads. The SendResult is discarded because nothing
	// here owns a root to record — a caller that needs one enqueues its own
	// bookkeeping alongside the send.
	_, err := p.sender.Send(ctx, payload.TransportName, payload.Target, entity.NotifyMessage{
		Subject:     payload.Subject,
		Body:        payload.Body,
		MessageMIME: payload.MessageMIME,
	}, payload.ReplyTo)
	if err != nil {
		xlog.Error(ctx, "messaging processor send failed", xfield.Error(err))
		return err
	}

	return nil
}
