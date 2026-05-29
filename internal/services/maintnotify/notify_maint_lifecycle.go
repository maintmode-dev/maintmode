package maintnotify

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (n *Service) NotifyMaintLifecycle(ctx context.Context, kind entity.NotifyEventKind, maint *entity.Maintenance) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.MaintNotify.NotifyMaintLifecycle",
		xfield.String("event", string(kind)),
		xfield.String("maintID", maint.ID.String()),
	)
	defer span.End()

	n.notifyMaintLifecycle(ctx, n.dispatchSync, entity.NotifyEvent{
		Kind:                kind,
		MaintID:             maint.ID,
		MaintTitle:          maint.Title,
		CancelReason:        maint.CancelReason,
		CancelReasonComment: maint.CancelReasonComment,
	})
}

func (n *Service) NotifyAsyncMaintLifecycle(ctx context.Context, kind entity.NotifyEventKind, maint *entity.Maintenance) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.MaintNotify.NotifyAsyncMaintLifecycle",
		xfield.String("event", string(kind)),
		xfield.String("maintID", maint.ID.String()),
	)
	defer span.End()

	n.notifyMaintLifecycle(ctx, n.dispatchAsync, entity.NotifyEvent{
		Kind:                kind,
		MaintID:             maint.ID,
		MaintTitle:          maint.Title,
		CancelReason:        maint.CancelReason,
		CancelReasonComment: maint.CancelReasonComment,
	})
}

func (n *Service) notifyMaintLifecycle(ctx context.Context,
	notifyFunc func(ctx context.Context, evt entity.NotifyEvent) error,
	event entity.NotifyEvent,
) {
	if event.Kind.IsStep() {
		xlog.Error(ctx, "invalid event",
			xfield.Error(fmt.Errorf("%s is a step event, not maintenance", event.Kind)),
		)
		return
	}

	err := notifyFunc(ctx, event)
	if err != nil {
		xlog.Error(ctx, "notify failed", xfield.Error(err))
	}
}
