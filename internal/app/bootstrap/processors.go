package bootstrap

import (
	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/messaging/taskprocessor"
)

func NewTaskProcessors(
	cfg config.TaskProcessorConfig,
	stores *Stores,
	services *Services,
	gateways *Gateways,
) *goque.Goque {
	_ = services

	goq := goque.NewGoque(stores.taskStorage)
	goq.RegisterProcessor(
		entity.ProcessorTaskMessagingSend,
		taskprocessor.NewMessagingTaskProcessor(gateways.NotifyTransportRegistry),
		goque.WithWorkersCount(cfg.Messaging.Workers),
		goque.WithTaskProcessingMaxAttempts(cfg.Messaging.MaxAttempts),
	)

	return goq
}
