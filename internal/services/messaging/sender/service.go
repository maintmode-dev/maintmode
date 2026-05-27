package sender

import (
	"github.com/ruko1202/goque"

	gwmsg "github.com/ruko1202/maintmode/internal/gateways/messengers"
)

// Service is the messaging facade.
type Service struct {
	messengersRegistry *gwmsg.MessengerRegistry
	queue              goque.TaskQueueManager
}

func NewMessengerService(
	messengersRegistry *gwmsg.MessengerRegistry,
	queue goque.TaskQueueManager,
) *Service {
	return &Service{
		messengersRegistry: messengersRegistry,
		queue:              queue,
	}
}
