package messagesender

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// SendAsync enqueues msg for delivery via goque under taskType. The task type
// selects which worker drains the task: callers pass
// entity.ProcessorTaskMessagingSend for ordinary notifications, or a dedicated
// type (e.g. ProcessorTaskInvitationEmailSend) to route work onto a queue a
// specific binary alone processes. The payload shape is the same, so one
// asyncsenderprocessor handles either task type. If ctx carries a *sqlx.Tx via
// dbtx.WithTx, the enqueue participates in that tx (outbox).
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
