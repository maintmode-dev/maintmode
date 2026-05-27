package maintnotify

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (n *Service) NotifyMaintLifecycle(ctx context.Context, kind entity.NotifyEventKind, maint *entity.Maintenance) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.MaintNotify.NotifyMaintLifecycle",
		xfield.String("event", string(kind)),
		xfield.String("maintID", maint.ID.String()),
	)
	defer span.End()

	if kind.IsStep() {
		return fmt.Errorf("%s is a step event, not maintenance", kind)
	}

	return n.dispatchSync(ctx, entity.NotifyEvent{
		Kind:                kind,
		MaintID:             maint.ID,
		MaintTitle:          maint.Title,
		CancelReason:        maint.CancelReason,
		CancelReasonComment: maint.CancelReasonComment,
	})
}

func (n *Service) NotifyAsyncMaintLifecycle(ctx context.Context, kind entity.NotifyEventKind, maint *entity.Maintenance) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.MaintNotify.NotifyAsyncMaintLifecycle",
		xfield.String("event", string(kind)),
		xfield.String("maintID", maint.ID.String()),
	)
	defer span.End()

	if kind.IsStep() {
		return fmt.Errorf("%s is a step event, not maintenance", kind)
	}

	return n.dispatchAsync(ctx, entity.NotifyEvent{
		Kind:                kind,
		MaintID:             maint.ID,
		MaintTitle:          maint.Title,
		CancelReason:        maint.CancelReason,
		CancelReasonComment: maint.CancelReasonComment,
	})
}
