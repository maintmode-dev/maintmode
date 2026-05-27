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
) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.MaintNotify.NotifyStepLifecycle",
		xfield.String("event", string(kind)),
		xfield.String("maintID", maint.ID.String()),
		xfield.String("stepID", step.ID.String()),
	)
	defer span.End()

	if !kind.IsStep() {
		return fmt.Errorf("%s is not a step event", kind)
	}

	return n.dispatchSync(ctx, entity.NotifyEvent{
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
) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.MaintNotify.NotifyAsyncStepLifecycle",
		xfield.String("event", string(kind)),
		xfield.String("maintID", maint.ID.String()),
	)
	defer span.End()

	if !kind.IsStep() {
		return fmt.Errorf("%s is not a step event", kind)
	}

	return n.dispatchAsync(ctx, entity.NotifyEvent{
		Kind:            kind,
		MaintID:         maint.ID,
		MaintTitle:      maint.Title,
		StepID:          step.ID,
		StepOrder:       step.Order,
		StepDescription: step.Description,
	})
}
