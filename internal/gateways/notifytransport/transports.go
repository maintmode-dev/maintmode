package notifytransport

import (
	"context"

	"github.com/ruko1202/maintmode/internal/entity"
)

type Transport interface {
	TransportID() entity.NotifyTransport
	// Send delivers msg to target. replyTo, when non-nil, asks the transport to
	// thread the message under that message; nil sends top-level.
	//
	// replyTo is an argument rather than a field on NotifyMessage because the
	// rendered message is shared by every target of one transport (see
	// renderPerTransport) while the root is per-target. A field would make the
	// "one message per transport" invariant depend on the map handing out
	// copies, and the first refactor to pointers or a parallel loop would
	// silently leak one channel's root into another's.
	//
	// Delivery outranks threading: a transport that cannot honor replyTo must
	// send the message flat and report it in SendResult, never fail the send.
	Send(ctx context.Context, target string, msg entity.NotifyMessage, replyTo *entity.MessageRef) (entity.SendResult, error)
}

// TransportResolver resolves a transport by name at delivery time. The runtime
// resolver (RUK-196: config+secrets from the DB) satisfies it, so the sender and
// async processor depend on this interface rather than a concrete registry.
type TransportResolver interface {
	Get(ctx context.Context, name entity.NotifyTransport) (Transport, error)
}
