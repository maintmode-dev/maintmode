package asyncsenderprocessor

import (
	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/gateways/notifytransport"
)

// NewTaskProcessor returns the goque TaskProcessor that delivers async/delayed messages
func NewTaskProcessor(
	notifyTransportRegistry notifytransport.TransportResolver,
) goque.TaskProcessor {
	return goque.NewTypedTaskProcessor[entity.ProcessorTaskPayloadEventNotify](
		newQueueProcessorProcessor(notifyTransportRegistry),
		goque.WithCancelTaskWhenPayloadDecodeError[entity.ProcessorTaskPayloadEventNotify](),
	)
}

type queueProcessor struct {
	notifyTransportRegistry notifytransport.TransportResolver
}

func newQueueProcessorProcessor(notifyTransportRegistry notifytransport.TransportResolver) *queueProcessor {
	return &queueProcessor{
		notifyTransportRegistry: notifyTransportRegistry,
	}
}
