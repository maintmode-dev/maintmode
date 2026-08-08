package asyncsenderprocessor

import (
	"context"

	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/entity"
)

// messageSender is the delivery facade this processor drives: it resolves the
// transport and sends. The processor stays thin — it only unpacks the payload
// and delegates.
type messageSender interface {
	Send(
		ctx context.Context,
		trName entity.NotifyTransport,
		target string,
		msg entity.NotifyMessage,
		replyTo *entity.MessageRef,
	) (entity.SendResult, error)
}

// NewTaskProcessor returns the goque TaskProcessor that delivers async/delayed messages
func NewTaskProcessor(
	sender messageSender,
) goque.TaskProcessor {
	return goque.NewTypedTaskProcessor[entity.ProcessorTaskPayloadEventNotify](
		newQueueProcessorProcessor(sender),
		goque.WithCancelTaskWhenPayloadDecodeError[entity.ProcessorTaskPayloadEventNotify](),
	)
}

type queueProcessor struct {
	sender messageSender
}

func newQueueProcessorProcessor(
	sender messageSender,
) *queueProcessor {
	return &queueProcessor{
		sender: sender,
	}
}
