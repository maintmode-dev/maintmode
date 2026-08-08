package messagesender

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// SendAsync enqueues msg for delivery via goque under taskType. The task type
// selects which worker drains the task, which is how a caller routes its
// messages onto a queue processed independently of everyone else's —
// entity.ProcessorTaskInvitationEmailSend is the live example. The payload
// shape does not vary with the type, so one asyncsenderprocessor handles any of
// them. If ctx carries a *sqlx.Tx via dbtx.WithTx, the enqueue participates in
// that tx (outbox).
//
// replyTo threads the message under an earlier one, or nil for a top-level
// message. It travels in the payload, so the caller that enqueues the task is
// the one that decides — the processor just delivers what it was handed.
func (s *Service) SendAsync(
	ctx context.Context,
	taskType string,
	trName entity.NotifyTransport,
	target string,
	msg entity.NotifyMessage,
	replyTo *entity.MessageRef,
	idempotencyKey string,
) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Messaging.SendAsync",
		xfield.String("task_type", taskType),
		xfield.String("transport", string(trName)),
		xfield.String("target", target),
	)
	defer span.End()

	_, err := s.scheduler.Schedule(ctx,
		taskType,
		entity.ProcessorTaskPayloadEventNotify{
			TransportName: trName,
			Target:        target,
			Subject:       msg.Subject,
			Body:          msg.Body,
			MessageMIME:   msg.MessageMIME,
			ReplyTo:       replyTo,
		},
		idempotencyKey,
	)
	if err != nil {
		return err
	}

	return nil
}
