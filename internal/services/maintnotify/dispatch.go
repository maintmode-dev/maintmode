package maintnotify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/maintnotify/router"
	"github.com/ruko1202/maintmode/internal/services/messaging/sender"
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
func (n *Service) dispatchSync(ctx context.Context, evt entity.NotifyEvent) error {
	return n.dispatch(ctx, evt, func(ctx context.Context, msg entity.Message, route router.Route) error {
		return n.sender.Send(ctx, route.Transport, route.Target, msg)
	})
}

func (n *Service) dispatchAsync(ctx context.Context, evt entity.NotifyEvent) error {
	return n.dispatch(ctx, evt, func(ctx context.Context, msg entity.Message, route router.Route) error {
		return n.sender.SendAsync(ctx, route.Transport, route.Target, msg,
			sender.WithIdempotencyKey(idempotencyKey(evt, route)),
		)
	})
}

// dispatch renders the message once and fans it out to all routes.
// Failures of individual routes are logged and swallowed; the caller
// always sees nil unless rendering itself broke.
func (n *Service) dispatch(
	ctx context.Context,
	evt entity.NotifyEvent,
	senderF func(ctx context.Context, msg entity.Message, route router.Route) error,
) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.MaintNotify.dispatch")
	defer span.End()

	routes := n.router.Resolve(evt.Kind)
	if len(routes) == 0 {
		return nil
	}

	evt = n.fillEvent(evt)

	msg, err := n.renderer.Render(ctx, evt)
	if err != nil {
		return fmt.Errorf("render %s: %w", evt.Kind, err)
	}

	for _, route := range routes {
		if err := senderF(ctx, msg, route); err != nil {
			xlog.Error(ctx, "notification delivery failed",
				xfield.String("event", string(evt.Kind)),
				xfield.String("transport", string(route.Transport)),
				xfield.String("target", route.Target),
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
func idempotencyKey(evt entity.NotifyEvent, route router.Route) string {
	h := sha256.New()

	_, _ = fmt.Fprintf(h,
		"maint|%s|%s|%s|%s|%s",
		evt.Kind, evt.MaintID, evt.StepID, route.Transport, route.Target,
	)

	return hex.EncodeToString(h.Sum(nil))
}
