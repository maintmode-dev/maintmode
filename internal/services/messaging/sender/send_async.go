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
// Maintenance notifications do not come through here: they are delivered inline
// by the notifier, and the task type they used to enqueue under is retired.
func (s *Service) SendAsync(
	ctx context.Context,
	taskType string,
	trName entity.NotifyTransport,
	target string,
	msg entity.NotifyMessage,
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
		},
		idempotencyKey,
	)
	if err != nil {
		return err
	}

	return nil
}
