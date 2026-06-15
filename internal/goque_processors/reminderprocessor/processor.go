// Package reminderprocessor handles maint.reminder goque tasks: at fire time it
// loads the maintenance, and (unless it's no longer scheduled) renders the
// reminder template and delivers it to the maintenance's current notify targets.
package reminderprocessor

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/entity"
)

// MaintLoader loads a maintenance for rendering the reminder.
type MaintLoader interface {
	GetMaint(ctx context.Context, maintID uuid.UUID) (*entity.Maintenance, error)
}

// Notifier renders and delivers the reminder to the maintenance's notify targets.
type Notifier interface {
	NotifyMaintReminder(ctx context.Context, maint *entity.Maintenance) error
}

type processor struct {
	maintLoader MaintLoader
	notifier    Notifier
}

func newQueueProcessor(
	maintLoader MaintLoader,
	notifier Notifier,
) *processor {
	return &processor{
		maintLoader: maintLoader,
		notifier:    notifier,
	}
}

// NewTaskProcessor returns the goque TaskProcessor for ProcessorTaskMaintReminder. A payload
// that fails to decode is canceled rather than retried.
func NewTaskProcessor(
	maintLoader MaintLoader,
	notifier Notifier,
) goque.TaskProcessor {
	return goque.NewTypedTaskProcessor[entity.ProcessorTaskPayloadMaintReminder](
		newQueueProcessor(maintLoader, notifier),
		goque.WithCancelTaskWhenPayloadDecodeError[entity.ProcessorTaskPayloadMaintReminder](),
	)
}
