package usersummary

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

// ResolveMentions resolves the additionally mentioned users of a maintenance to
// everything a notification needs to address them. The result follows the input
// order, deduplicated, with the zero id dropped; an empty result means there is
// no mention line to render.
//
// The missing error return is the contract, not an oversight — the same
// reasoning as ResolveOwner. dispatchSync has no idempotency key, and an error
// traveling back out of NotifyMaintReminder is what makes goque retry the task,
// which re-sends to every target. A resolver able to fail could therefore turn
// "we could not name someone" into duplicate notifications, precisely when auth
// is unavailable. resolveUsers already degrades instead of failing; this
// signature fixes that property in the type system.
//
// Anyone who cannot actually be pinged is dropped from the list — blocked users
// and ids that do not resolve alike. ResolveOwner keeps both, labeled
// "[blocked]" or "Unknown user", and that asymmetry is deliberate: the owner is
// the person accountable for the maintenance, so a channel reading the message
// needs to know the system could not name them. A mention exists only to ping
// someone. "Unknown user" pings nobody and tells the reader nothing, so putting
// it in the line adds noise instead of information.
func (s *Service) ResolveMentions(ctx context.Context, ids []uuid.UUID) []*entity.UserMention {
	uniqIDs := lo.Filter(lo.Uniq(ids), func(item uuid.UUID, _ int) bool {
		return item != uuid.Nil
	})
	if len(uniqIDs) == 0 {
		return nil
	}

	ctx, span := xlog.WithOperationSpan(ctx, "service.Usersummary.ResolveMentions")
	defer span.End()

	users := s.resolveUsers(ctx, uniqIDs)

	mentions := make([]*entity.UserMention, 0, len(uniqIDs))

	for _, id := range uniqIDs {
		user := users[id]
		if user == nil {
			// Auth is down or the user is gone. Either way there is no handle
			// and no name to print, so the entry is dropped rather than
			// rendered as an unknown placeholder nobody can act on.
			xlog.Warn(ctx, "mention unresolved, dropping it", xfield.String("id", id.String()))

			continue
		}

		if user.IsBlocked() {
			continue
		}

		mentions = append(mentions, &entity.UserMention{
			Name:        user.Name,
			TelegramTag: user.TelegramTag,
			SlackTag:    user.SlackTag,
		})
	}

	return mentions
}
