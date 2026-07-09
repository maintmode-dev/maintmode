package notifytransport

import (
	"context"

	"github.com/ruko1202/maintmode/internal/entity"
)

type Transport interface {
	TransportID() entity.NotifyTransport
	Send(ctx context.Context, target string, msg entity.NotifyMessage) error
}

// TransportResolver resolves a transport by name at delivery time. The runtime
// resolver (RUK-196: config+secrets from the DB) satisfies it, so the sender and
// async processor depend on this interface rather than a concrete registry.
type TransportResolver interface {
	Get(ctx context.Context, name entity.NotifyTransport) (Transport, error)
}
