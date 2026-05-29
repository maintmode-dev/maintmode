package taskprocessor

import (
	"context"
	"fmt"

	"github.com/ruko1202/goque"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (p *processor) ProcessTask(ctx context.Context, task *goque.TypedTask[entity.ProcessorTaskPayloadEventNotify]) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Messaging.Processor.ProcessTask",
		xfield.String("transport", string(task.Payload.TransportName)),
		xfield.String("target", task.Payload.Target),
	)
	defer span.End()

	payload := task.Payload

	transport, err := p.notifyTransportRegistry.Get(ctx, payload.TransportName)
	if err != nil {
		return fmt.Errorf("messaging processor: no transport %q: %w", payload.TransportName, err)
	}

	err = transport.Send(ctx, payload.Target, entity.NotifyMessage{
		Subject: payload.Subject,
		Body:    payload.Body,
	})
	if err != nil {
		xlog.Error(ctx, "messaging processor send failed", xfield.Error(err))
		return err
	}

	return nil
}
