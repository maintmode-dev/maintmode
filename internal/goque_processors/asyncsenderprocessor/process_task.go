package asyncsenderprocessor

import (
	"context"
	"errors"
	"fmt"

	"github.com/ruko1202/goque"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func (p *queueProcessor) ProcessTask(ctx context.Context, task *goque.TypedTask[entity.ProcessorTaskPayloadEventNotify]) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Messaging.QueueProcessor.ProcessTask",
		xfield.String("transport", string(task.Payload.TransportName)),
		xfield.String("target", task.Payload.Target),
	)
	defer span.End()

	payload := task.Payload

	transport, err := p.notifyTransportRegistry.Get(ctx, payload.TransportName)
	if err != nil {
		// A disabled or unconfigured integration is a best-effort drop, not a
		// failure: return nil so goque acks the task instead of retrying it to
		// the dead-letter queue. This matches the synchronous dispatch path, and
		// matters now that resolving a disabled integration (RUK-196) is a
		// routine Get error rather than a rare unsupported-transport error.
		if errors.Is(err, apperr.ErrIntegrationDisabled) {
			xlog.Warn(ctx, "messaging processor: integration disabled, dropping delivery",
				xfield.String("transport", string(payload.TransportName)))
			return nil
		}
		return fmt.Errorf("messaging processor: no transport %q: %w", payload.TransportName, err)
	}

	err = transport.Send(ctx, payload.Target, entity.NotifyMessage{
		Subject:     payload.Subject,
		Body:        payload.Body,
		MessageMIME: payload.MessageMIME,
	})
	if err != nil {
		xlog.Error(ctx, "messaging processor send failed", xfield.Error(err))
		return err
	}

	return nil
}
