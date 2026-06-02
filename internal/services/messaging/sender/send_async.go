package messagesender

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// SendAsync enqueues msg in goque. If ctx carries a *sqlx.Tx via
// dbtx.WithTx, the enqueue participates in that tx (outbox)
func (s *Service) SendAsync(
	ctx context.Context,
	trName entity.NotifyTransport,
	target string,
	msg entity.NotifyMessage,
	idempotencyKey string,
) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Messaging.SendAsync",
		xfield.String("transport", string(trName)),
		xfield.String("target", target),
	)
	defer span.End()

	_, err := s.scheduler.ScheduleDelayed(ctx,
		entity.ProcessorTaskMessagingSend,
		entity.ProcessorTaskPayloadEventNotify{
			TransportName: trName,
			Target:        target,
			Subject:       msg.Subject,
			Body:          msg.Body,
		},
		0,
		idempotencyKey,
	)
	if err != nil {
		return err
	}

	return nil
}
