package taskprocessor

import (
	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/gateways/notifytransport"
)

// NewMessagingTaskProcessor returns the goque TaskProcessor that delivers async/delayed messages
func NewMessagingTaskProcessor(
	notifyTransportRegistry *notifytransport.Registry,
) goque.TaskProcessor {
	return goque.NewTypedTaskProcessor[entity.ProcessorTaskPayloadEventNotify](
		newProcessor(notifyTransportRegistry),
		goque.WithCancelTaskWhenPayloadDecodeError[entity.ProcessorTaskPayloadEventNotify](),
	)
}

type processor struct {
	notifyTransportRegistry *notifytransport.Registry
}

func newProcessor(notifyTransportRegistry *notifytransport.Registry) *processor {
	return &processor{
		notifyTransportRegistry: notifyTransportRegistry,
	}
}
