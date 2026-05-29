package maintnotify

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (n *Service) NotifyStepLifecycle(
	ctx context.Context,
	kind entity.NotifyEventKind,
	maint *entity.Maintenance,
	step *entity.MaintenanceStep,
) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.MaintNotify.NotifyStepLifecycle",
		xfield.String("event", string(kind)),
		xfield.String("maintID", maint.ID.String()),
		xfield.String("stepID", step.ID.String()),
	)
	defer span.End()

	n.notifyStepLifecycle(ctx, n.dispatchSync, entity.NotifyEvent{
		Kind:            kind,
		MaintID:         maint.ID,
		MaintTitle:      maint.Title,
		StepID:          step.ID,
		StepOrder:       step.Order,
		StepDescription: step.Description,
	})
}

func (n *Service) NotifyAsyncStepLifecycle(
	ctx context.Context,
	kind entity.NotifyEventKind,
	maint *entity.Maintenance,
	step *entity.MaintenanceStep,
) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.MaintNotify.NotifyAsyncStepLifecycle",
		xfield.String("event", string(kind)),
		xfield.String("maintID", maint.ID.String()),
	)
	defer span.End()

	n.notifyStepLifecycle(ctx, n.dispatchAsync, entity.NotifyEvent{
		Kind:            kind,
		MaintID:         maint.ID,
		MaintTitle:      maint.Title,
		StepID:          step.ID,
		StepOrder:       step.Order,
		StepDescription: step.Description,
	})
}

func (n *Service) notifyStepLifecycle(ctx context.Context,
	notifyFunc func(ctx context.Context, evt entity.NotifyEvent) error,
	event entity.NotifyEvent,
) {
	if !event.Kind.IsStep() {
		xlog.Error(ctx, "invalid event",
			xfield.Error(fmt.Errorf("%s is not a step event", event.Kind)),
		)
		return
	}

	err := notifyFunc(ctx, event)
	if err != nil {
		xlog.Error(ctx, "notify failed", xfield.Error(err))
	}
}
