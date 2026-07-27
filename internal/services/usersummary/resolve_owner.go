package usersummary

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// blockedNameSuffix is appended to a blocked user's display name in a mention.
// The label is deliberately visible in the notification body: it tells the
// channel that the person nominally responsible has no access anymore and a
// stand-in is needed. Silently degrading to the bare name would lose that
// signal.
const blockedNameSuffix = " [blocked]"

// ResolveOwner resolves a single user id to everything a notification needs to
// address that person: display name plus messenger handles. The zero id yields
// nil ("no owner to mention" — step events legitimately carry uuid.Nil and must
// not render an "Unknown user" owner). For any non-nil id it always returns a
// value, never nil and never an error: an id that cannot be resolved (unknown
// user, or the user service unavailable) degrades to the labeled
// entity.UnknownUserName with no tags.
//
// This is a full resolver, not a cache getter. Its main caller is the reminder
// task, which runs in the background queue with no preceding HTTP read, so the
// cache — populated only by ResolveMany — is normally cold there. A cache miss
// therefore fetches from the user service and warms the cache; treating a miss
// as "unresolved" would silently disable the feature almost always.
//
// A blocked user gets the name with a "[blocked]" label and NO tags: pinging
// someone whose access was revoked is pointless, and the label surfaces that the
// owner is unavailable. getUsersByIDs deliberately does not exclude blocked
// users (read paths still want to name them), so the filtering happens here.
func (s *Service) ResolveOwner(ctx context.Context, id uuid.UUID) *entity.UserMention {
	if id == uuid.Nil {
		return nil
	}

	ctx, span := xlog.WithOperationSpan(ctx, "service.Usersummary.ResolveOwner")
	defer span.End()

	user := s.resolveUsers(ctx, []uuid.UUID{id})[id]
	if user == nil {
		xlog.Warn(ctx, "owner mention unresolved, using unknown label", xfield.String("id", id.String()))
		return &entity.UserMention{Name: entity.UnknownUserName}
	}

	if user.IsBlocked() {
		return &entity.UserMention{Name: fmt.Sprintf("%s%s", user.Name, blockedNameSuffix)}
	}

	return &entity.UserMention{
		Name:        user.Name,
		TelegramTag: user.TelegramTag,
		SlackTag:    user.SlackTag,
	}
}
