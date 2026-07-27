package maintnotify

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/maintnotify/render"
)

// MessageSender delivers a rendered message to one transport address.
//
// Only the inline Send is declared here. The queue-backed SendAsync still
// exists on the sender service — the invitation e-mail path uses it — but
// maintenance notifications no longer go through the queue, so this consumer
// does not depend on it.
type MessageSender interface {
	Send(ctx context.Context, trName entity.NotifyTransport, target string, msg entity.NotifyMessage) error
}

type NotifyTargetsStore interface {
	ListByMaint(ctx context.Context, maintID uuid.UUID) ([]*entity.NotifyTarget, error)
}

// OwnerResolver turns a user id into everything needed to mention that person.
//
// The missing error return is the contract, not an oversight. dispatchSync has
// no idempotency key, and an error returned from NotifyMaintReminder is the
// signal that makes goque retry the task — which re-sends to every target. A
// resolver that could fail would therefore be able to turn "we could not name
// the owner" into duplicate notifications. Declaring it without an error makes
// that impossible by construction: an unresolvable id degrades to a labeled
// name inside the returned mention, and the zero id yields nil.
type OwnerResolver interface {
	ResolveOwner(ctx context.Context, id uuid.UUID) *entity.UserMention
}

// EventRenderer turns an event into the message body for one transport. It is
// an interface rather than the concrete *render.Service so tests can drive the
// "one transport renders, the next fails" case — the ordering hazard that the
// render-everything-first rule in dispatch exists to prevent. No real input can
// produce it, because a render failure depends on the event kind and the
// frontend URL, both transport-independent.
type EventRenderer interface {
	Render(ctx context.Context, transport entity.NotifyTransport, evt entity.NotifyEvent) (entity.NotifyMessage, error)
}

// Service turns maintenance-lifecycle events into Sender.Send calls
type Service struct {
	frontendURL   string
	notifyTargets NotifyTargetsStore
	renderer      EventRenderer
	sender        MessageSender
	ownerResolver OwnerResolver
}

func NewNotifier(
	cfg *config.AppConfig,
	sender MessageSender,
	notifyTargets NotifyTargetsStore,
	ownerResolver OwnerResolver,
) (*Service, error) {
	rend, err := render.New()
	if err != nil {
		return nil, fmt.Errorf("init renderer: %w", err)
	}

	return &Service{
		frontendURL:   cfg.App.FrontendURL,
		notifyTargets: notifyTargets,
		renderer:      rend,
		sender:        sender,
		ownerResolver: ownerResolver,
	}, nil
}
