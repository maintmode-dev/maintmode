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

// MentionResolver turns the mentioned user ids into renderable mentions.
//
// It is declared separately from OwnerResolver rather than added to it because
// the two degrade differently — a blocked owner stays named, a blocked mention is
// dropped — and because widening OwnerResolver would force every existing
// owner-path test double to grow a method it does not exercise. Both are
// satisfied by *usersummary.Service.
//
// The absent error return is the same contract as OwnerResolver's, for the same
// reason: an error here would become a goque retry and re-send to every target.
type MentionResolver interface {
	ResolveMentions(ctx context.Context, ids []uuid.UUID) []*entity.UserMention
}

// MentionsReader reads the ids of the users a maintenance mentions.
//
// The signature deliberately speaks only uuid.UUID. Mentions live in
// storages/maintenances, a core store that depguard forbids the notificator
// module from importing; declaring the dependency consumer-side keeps the import
// out of this package while bootstrap supplies the concrete *maintenances.Store.
// It is satisfied by the store directly rather than by the maint service, which
// already holds this notifier — routing through it would close an import cycle.
type MentionsReader interface {
	GetMaintMentions(ctx context.Context, maintID uuid.UUID) ([]uuid.UUID, error)
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
	frontendURL     string
	notifyTargets   NotifyTargetsStore
	renderer        EventRenderer
	sender          MessageSender
	ownerResolver   OwnerResolver
	mentions        MentionsReader
	mentionResolver MentionResolver
}

func NewNotifier(
	cfg *config.AppConfig,
	sender MessageSender,
	notifyTargets NotifyTargetsStore,
	ownerResolver OwnerResolver,
	mentions MentionsReader,
	mentionResolver MentionResolver,
) (*Service, error) {
	rend, err := render.New()
	if err != nil {
		return nil, fmt.Errorf("init renderer: %w", err)
	}

	return &Service{
		frontendURL:     cfg.App.FrontendURL,
		notifyTargets:   notifyTargets,
		renderer:        rend,
		sender:          sender,
		ownerResolver:   ownerResolver,
		mentions:        mentions,
		mentionResolver: mentionResolver,
	}, nil
}
