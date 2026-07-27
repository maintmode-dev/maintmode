package maintnotify

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/metrics"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// dispatchSync is best-effort: notification delivery never fails the
// caller. It is called post-commit, when the lifecycle transition is
// already durable, so surfacing a notify error to the API client whose
// business operation already succeeded would be misleading.
//
// It has no idempotency key: sender.Send delivers inline rather than
// through the queue, so a caller that retries — notably the reminder
// processor, which turns a returned error into a goque retry — re-sends
// to every target. Any code path added inside dispatch must therefore
// avoid returning an error after the send loop has begun.
//
// The "no blocking network call inside a DB tx" invariant is enforced
// one layer below in sender.Send — it refuses to run when ctx carries
// a tx. dispatchSync inherits that guard transitively.
func (n *Service) dispatchSync(ctx context.Context, event entity.NotifyEvent) error {
	return n.dispatch(ctx, event, func(ctx context.Context, msg entity.NotifyMessage, target *entity.NotifyTarget) error {
		return n.sender.Send(ctx, target.Transport, target.TransportChannelID, msg)
	})
}

// dispatch renders the message once per transport and fans it out to all routes.
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

	// Step events leave CreatedByUserID zero, so they skip this without the
	// dispatch code ever naming them. The resolver cannot fail by contract (see
	// OwnerResolver), so this adds no error path to the send loop below.
	if event.CreatedByUserID != uuid.Nil {
		event.OwnerMention = n.ownerResolver.ResolveOwner(ctx, event.CreatedByUserID)
	}

	messages, err := n.renderPerTransport(ctx, event, notifyTargets)
	if err != nil {
		metrics.MaintNotifyDispatchRenderError(ctx)
		return fmt.Errorf("render %s: %w", event.Kind, err)
	}

	for _, notifyTarget := range notifyTargets {
		if err := senderF(ctx, messages[notifyTarget.Transport], notifyTarget); err != nil {
			xlog.Error(ctx, "notification delivery failed",
				xfield.String("transport", string(notifyTarget.Transport)),
				xfield.String("channel_id", notifyTarget.ChannelID.String()),
				xfield.String("channel", notifyTarget.TransportChannelID),
				xfield.Error(err))
		}
	}

	return nil
}

// renderPerTransport renders one message per distinct transport among the
// targets — transports number two, targets number N — and returns them all or
// nothing.
//
// Rendering every transport before the send loop starts is the invariant, not an
// optimization. Rendering lazily inside the loop would let transport A receive
// its message, transport B fail to render, and dispatch return an error that
// makes goque retry the whole task — delivering to A twice, since dispatchSync
// carries no idempotency key. "A render failure means zero sends" only holds
// while every render happens first.
//
// The map is keyed by the transport itself, so distinct transports can never
// share an entry. A target whose Transport is empty — the zero value carried by
// instances built from persisted columns alone, which ListByMaint should not
// produce — groups only with other empty ones and renders through the name
// fallback. It is logged and still delivered: the mention is a decoration, and
// dropping a recipient over it would be a worse failure.
func (n *Service) renderPerTransport(
	ctx context.Context,
	event entity.NotifyEvent,
	notifyTargets []*entity.NotifyTarget,
) (map[entity.NotifyTransport]entity.NotifyMessage, error) {
	messages := make(map[entity.NotifyTransport]entity.NotifyMessage, len(notifyTargets))

	for _, notifyTarget := range notifyTargets {
		if _, ok := messages[notifyTarget.Transport]; ok {
			continue
		}

		if notifyTarget.Transport == "" {
			xlog.Warn(ctx, "notify target has no transport, owner mention falls back to name",
				xfield.String("channel_id", notifyTarget.ChannelID.String()))
		}

		msg, err := n.renderer.Render(ctx, notifyTarget.Transport, event)
		if err != nil {
			return nil, err
		}

		messages[notifyTarget.Transport] = msg
	}

	return messages, nil
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
