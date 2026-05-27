package taskprocessor

import (
	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/entity"
	gwmsg "github.com/ruko1202/maintmode/internal/gateways/messengers"
)

// NewMessagingTaskProcessor returns the goque TaskProcessor that delivers async/delayed messages
func NewMessagingTaskProcessor(
	messengersRegistry *gwmsg.MessengerRegistry,
) goque.TaskProcessor {
	return goque.NewTypedTaskProcessor[entity.ProcessorTaskPayloadEventNotify](
		newProcessor(messengersRegistry),
		goque.WithCancelTaskWhenPayloadDecodeError[entity.ProcessorTaskPayloadEventNotify](),
	)
}

type processor struct {
	messengersRegistry *gwmsg.MessengerRegistry
}

func newProcessor(messengersRegistry *gwmsg.MessengerRegistry) *processor {
	return &processor{
		messengersRegistry: messengersRegistry,
	}
}
