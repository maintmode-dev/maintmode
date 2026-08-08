package maintnotify

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

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
	return n.dispatch(ctx, event, func(
		ctx context.Context,
		msg entity.NotifyMessage,
		target *entity.NotifyTarget,
		replyTo *entity.MessageRef,
	) (entity.SendResult, error) {
		return n.sender.Send(ctx, target.Transport, target.TransportChannelID, msg, replyTo)
	})
}

// dispatch renders the message once per transport and fans it out to all routes.
func (n *Service) dispatch(
	ctx context.Context,
	event entity.NotifyEvent,
	senderF func(
		ctx context.Context,
		msg entity.NotifyMessage,
		target *entity.NotifyTarget,
		replyTo *entity.MessageRef,
	) (entity.SendResult, error),
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

	event.Mentions = n.resolveMentions(ctx, event)

	messages, err := n.renderPerTransport(ctx, event, notifyTargets)
	if err != nil {
		metrics.MaintNotifyDispatchRenderError(ctx)
		return fmt.Errorf("render %s: %w", event.Kind, err)
	}

	for _, notifyTarget := range notifyTargets {
		// The message is taken from the map BY VALUE and never written back: it
		// is shared by every target of this transport, and the thread root is
		// per-target. That is why replyTo travels as its own argument.
		res, err := senderF(ctx, messages[notifyTarget.Transport], notifyTarget, n.replyTo(ctx, event, notifyTarget))
		if err != nil {
			xlog.Error(ctx, "notification delivery failed",
				xfield.String("transport", string(notifyTarget.Transport)),
				xfield.String("channel_id", notifyTarget.ChannelID.String()),
				xfield.String("channel", notifyTarget.TransportChannelID),
				xfield.Error(err))

			continue
		}

		n.recordRoot(ctx, event, notifyTarget, res)
	}

	return nil
}

// replyTo picks the thread root this target's message should answer.
//
// maint.started is the root itself, and maint.reminder fires before the
// maintenance starts — both go out top-level. Everything else replies to the
// root recorded for THIS channel, so two channels never cross threads.
func (n *Service) replyTo(
	ctx context.Context,
	event entity.NotifyEvent,
	target *entity.NotifyTarget,
) *entity.MessageRef {
	if event.Kind == entity.NotifyEventMaintStarted || event.Kind == entity.NotifyEventMaintReminder {
		return nil
	}

	// No root is the normal state for maintenances that started before threading
	// existed: they live out their lifetime flat.
	if target.RootRef == nil {
		return nil
	}

	// The catalog address is editable at runtime while the root is frozen at
	// start time. Replying into a re-pointed channel would answer message 42 of
	// a chat where message 42 belongs to strangers — Telegram resolves reply ids
	// in the destination chat, and AllowSendingWithoutReply does not help
	// because such a reply is formally valid.
	//
	// The comparison is against the address the root was DELIVERED to, not
	// against the chat id the API answered with. Those two are written by
	// different systems and legitimately differ: transport_channel_id is
	// free-form operator text ("@ops_alerts", "#ops") validated only for
	// non-emptiness, while the API returns a canonical id ("-1001234567890",
	// "C0DEADBEEF"). Comparing them would find a mismatch on every event of
	// every channel not addressed by its exact canonical id — silently
	// disabling threading for a supported configuration.
	if target.RootChannel != "" && target.RootChannel != target.TransportChannelID {
		xlog.Warn(ctx, "notify channel was re-pointed after the root was recorded, sending flat",
			xfield.String("channel_id", target.ChannelID.String()),
			xfield.String("root_channel", target.RootChannel),
			xfield.String("channel", target.TransportChannelID))

		return nil
	}

	return target.RootRef
}

// recordRoot persists the thread root: the maintenance-started message, or the
// message that had to be posted without a root after the transport rejected the
// old one.
//
// Re-seeding on rejection rather than dropping the root is deliberate. A dropped
// root sends every remaining event of that maintenance flat, which is exactly
// the scattered-channel problem threading exists to solve. Re-seeding restarts
// the thread from the message that broke it: the new root is titled "step 4
// completed" rather than the maintenance, but the events after it are grouped
// again, and every message carries the maintenance title and its link anyway.
//
// A permanently unthreadable channel makes this cost one extra API call per
// event — the new root is rejected the same way the old one was. That is the
// accepted price for recovering the two cases that do heal: a deleted root and
// an unlocked thread.
//
// Every failure here is logged, never returned. dispatch has no idempotency key,
// so an error escaping after the send loop began becomes a goque retry that
// re-delivers to every target: a lost root costs a flat thread, a duplicate
// notification is worse. For the same reason one target's failure does not stop
// the loop — the caller keeps iterating.
func (n *Service) recordRoot(
	ctx context.Context,
	event entity.NotifyEvent,
	target *entity.NotifyTarget,
	res entity.SendResult,
) {
	// Two messages become the root: the maintenance-started one, and whichever
	// message a transport had to deliver after refusing the previous root.
	//
	// RootReplaced is a fact reported by the transport, never inferred from ids:
	// a successful threaded reply carries its own new id, so "returned ref is not
	// the root" would be true on success too and would re-seed the root on every
	// event.
	isNewRoot := event.Kind == entity.NotifyEventMaintStarted || res.RootReplaced
	if !isNewRoot || res.Ref == nil {
		return
	}

	// The address delivered to is stored alongside the message id so a later
	// re-point of this channel is detectable.
	if err := n.notifyTargets.SetRootRef(
		ctx, event.MaintID, target.ChannelID, res.Ref.MessageID, target.TransportChannelID,
	); err != nil {
		xlog.Error(ctx, "failed to record thread root, later events go out flat",
			xfield.String("maintID", event.MaintID.String()),
			xfield.String("channel_id", target.ChannelID.String()),
			xfield.Error(err))
	}
}

// resolveMentions loads the maintenance's mentioned user ids and turns them into
// renderable mentions, once per event and before the first target is contacted.
//
// The load happens HERE rather than in the event constructors because those have
// already flattened the maintenance to scalars — NotifyMaintReminder in
// particular receives a maintenance the reminder processor read from the raw
// store, which hydrates no child collections. A caller-supplied list would
// therefore be silently empty on the reminder path.
//
// A load failure is deliberately NOT returned. dispatchSync carries no
// idempotency key, so any error escaping this function after the send loop can
// begin turns into a goque retry that re-delivers to every target — a duplicate
// notification is a far worse outcome than a missing decoration. The failure is
// logged instead, and the message goes out without mentions.
func (n *Service) resolveMentions(ctx context.Context, event entity.NotifyEvent) []*entity.UserMention {
	// Step notifications never carry mentions, so they skip the query entirely
	// rather than paying for it on every step of every maintenance.
	if event.Kind.IsStep() {
		return nil
	}

	ids, err := n.mentions.GetMaintMentions(ctx, event.MaintID)
	if err != nil {
		// Logged, not counted on the dispatch-error metric: that one means the
		// notification was dropped, and this one still goes out — just without
		// its mention line.
		xlog.Error(ctx, "failed to load maintenance mentions, sending without them",
			xfield.String("maintID", event.MaintID.String()),
			xfield.Error(err))

		return nil
	}

	// The owner is dropped by id, before the resolve: entity.UserMention carries
	// no ID, so neither the resolver nor the renderer could deduplicate later —
	// they would have to compare by name (breaks on namesakes) or by handle
	// (breaks when there is none).
	ids = lo.Filter(ids, func(id uuid.UUID, _ int) bool {
		return id != event.CreatedByUserID
	})
	if len(ids) == 0 {
		return nil
	}

	return n.mentionResolver.ResolveMentions(ctx, ids)
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
