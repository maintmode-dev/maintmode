package bootstrap

import (
	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/messaging/goque_processors/asyncsenderprocessor"
	"github.com/ruko1202/maintmode/internal/services/messaging/goque_processors/reminderprocessor"
)

func NewTaskProcessors(
	cfg config.TaskProcessorConfig,
	stores *Stores,
	services *Services,
	gateways *Gateways,
) *goque.Goque {
	goq := goque.NewGoque(stores.taskStorage)
	goq.RegisterProcessor(
		entity.ProcessorTaskMessagingSend,
		asyncsenderprocessor.NewTaskProcessor(gateways.NotifyTransportRegistry),
		goque.WithWorkersCount(cfg.Messaging.Workers),
		goque.WithTaskProcessingMaxAttempts(cfg.Messaging.MaxAttempts),
	)

	// maint.reminder tasks resolve the maintenance's current notify targets and
	// render the reminder at fire time, so they share the maint store + notifier
	// rather than carrying a pre-rendered payload.
	goq.RegisterProcessor(
		entity.ProcessorTaskMaintReminder,
		reminderprocessor.NewTaskProcessor(stores.Maintenances, services.Notifier),
		goque.WithWorkersCount(cfg.Messaging.Workers),
		goque.WithTaskProcessingMaxAttempts(cfg.Messaging.MaxAttempts),
	)

	return goq
}
