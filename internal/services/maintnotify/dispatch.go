package maintnotify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	messagesender "github.com/ruko1202/maintmode/internal/services/messaging/sender"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/metrics"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// Both dispatchSync and dispatchAsync are best-effort: notification
// delivery never fails the caller. sync is called post-commit (lifecycle
// already durable); async is fire-and-forget. In neither case it makes
// sense to surface a notify error to the API client whose business
// operation already succeeded.
//
// The "no blocking network call inside a DB tx" invariant is enforced
// one layer below in sender.Send — it refuses to run when ctx carries
// a tx. dispatchSync inherits that guard transitively.
func (n *Service) dispatchSync(ctx context.Context, event entity.NotifyEvent) error {
	return n.dispatch(ctx, event, func(ctx context.Context, msg entity.NotifyMessage, target *entity.NotifyTarget) error {
		return n.sender.Send(ctx, target.Transport, target.ChannelID, msg)
	})
}

func (n *Service) dispatchAsync(ctx context.Context, event entity.NotifyEvent) error {
	return n.dispatch(ctx, event, func(ctx context.Context, msg entity.NotifyMessage, target *entity.NotifyTarget) error {
		return n.sender.SendAsync(ctx, target.Transport, target.ChannelID, msg,
			messagesender.WithIdempotencyKey(idempotencyKey(event, target)),
		)
	})
}

// dispatch renders the message once and fans it out to all routes.
func (n *Service) dispatch(
	ctx context.Context,
	event entity.NotifyEvent,
	senderF func(ctx context.Context, msg entity.NotifyMessage, target *entity.NotifyTarget) error,
) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.MaintNotify.dispatch")
	defer span.End()

	notifyTargets, err := n.notifyTargets.ListByMaint(ctx, event.MaintID)
	if err != nil {
		// Swallowed upstream (lifecycle already committed), so this
		// drop is otherwise invisible — count it for alerting.
		metrics.MaintNotifyDispatchResolveError(ctx)
		return fmt.Errorf("failed to list notify targets: %w", err)
	}
	if len(notifyTargets) == 0 {
		// No recipients — nothing to render or send.
		return nil
	}

	event = n.fillEvent(event)

	msg, err := n.renderer.Render(ctx, event)
	if err != nil {
		metrics.MaintNotifyDispatchRenderError(ctx)
		return fmt.Errorf("render %s: %w", event.Kind, err)
	}

	for _, notifyTarget := range notifyTargets {
		if err := senderF(ctx, msg, notifyTarget); err != nil {
			xlog.Error(ctx, "notification delivery failed",
				xfield.String("transport", string(notifyTarget.Transport)),
				xfield.String("channel", notifyTarget.ChannelID),
				xfield.Error(err))
		}
	}

	return nil
}

// fillEvent populates derived fields (OccurredAt, FrontendURL) when the
// caller didn't supply them.
func (n *Service) fillEvent(evt entity.NotifyEvent) entity.NotifyEvent {
	if evt.OccurredAt.IsZero() {
		evt.OccurredAt = xtime.UTCNow()
	}

	if evt.FrontendURL == "" {
		evt.FrontendURL = n.frontendURL
	}

	return evt
}

// idempotencyKey makes goque's unique (type, external_id) index collapse
// retries of the same (event, maint, step, route) tuple.
func idempotencyKey(evt entity.NotifyEvent, targets *entity.NotifyTarget) string {
	h := sha256.New()

	_, _ = fmt.Fprintf(h,
		"maint|%s|%s|%s|%s|%s",
		evt.Kind, evt.MaintID, evt.StepID, targets.Transport, targets.ChannelID,
	)

	return hex.EncodeToString(h.Sum(nil))
}
