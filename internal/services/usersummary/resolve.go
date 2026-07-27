package usersummary

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

// ResolveOne resolves a single user id to its summary. The zero id yields nil
// (no author to render). It never returns an error: an unresolved id degrades to
// the labeled "Unknown user" summary so the read does not fail.
func (s *Service) ResolveOne(ctx context.Context, id uuid.UUID) *entity.UserSummary {
	if id == uuid.Nil {
		return nil
	}
	return s.ResolveMany(ctx, []uuid.UUID{id})[id]
}

// ResolveMany batch-resolves user ids to summaries, indexed by id. It serves
// cache hits, fetches the misses from the user service in one query, and labels
// any id it could not resolve (lookup failure or unknown user) with the degraded
// "Unknown user" summary. The result contains an entry for every non-nil input
// id, so callers can map a list row's author without a nil check. It never
// returns an error — degrade, don't fail the read.
func (s *Service) ResolveMany(ctx context.Context, ids []uuid.UUID) map[uuid.UUID]*entity.UserSummary {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Usersummary.ResolveMany")
	defer span.End()

	users := s.resolveUsers(ctx, ids)

	out := make(map[uuid.UUID]*entity.UserSummary, len(users))
	for id, user := range users {
		if user == nil {
			// Unknown user or auth down: label and move on. The degraded entry is
			// kept so every non-nil input id has a result.
			xlog.Warn(ctx, "user summary unresolved, using unknown label", xfield.String("id", id.String()))
			out[id] = entity.NewUnknownUserSummary(id)
			continue
		}
		out[id] = user.ToUserSummary()
	}

	return out
}

// resolveUsers is the shared resolution layer behind ResolveMany and
// ResolveOwner: it dedupes the ids, serves cache hits, fetches the misses from
// the user service in one query and caches what came back. It returns the RAW
// users — not summaries — because ResolveOwner needs the messenger tags, which
// UserSummary deliberately does not carry (it is the privacy-safe API view).
//
// The map holds an entry for every non-nil input id, with a nil value for the
// ids that could not be resolved (unknown user, or the user service failing).
// Each caller decides how to degrade that nil, so ResolveMany keeps its exact
// prior behavior. Nothing here returns an error: a failing lookup is logged and
// treated as "resolved nothing" so reads and notifications degrade instead of
// failing. Unresolved ids are not cached — an id may resolve once auth recovers
// or the user is recreated.
func (s *Service) resolveUsers(ctx context.Context, ids []uuid.UUID) map[uuid.UUID]*entity.User {
	uniqIDs := lo.Filter(lo.Uniq(ids), func(item uuid.UUID, _ int) bool {
		return item != uuid.Nil
	})

	out := make(map[uuid.UUID]*entity.User, len(uniqIDs))
	if len(uniqIDs) == 0 {
		return out
	}

	// Serve cache hits; collect the misses to fetch from the user service in one query.
	misses := make([]uuid.UUID, 0, len(uniqIDs))
	for _, id := range uniqIDs {
		if item := s.cache.Get(id); item != nil {
			out[id] = item.Value()
			continue
		}
		misses = append(misses, id)
	}

	if len(misses) == 0 {
		return out
	}

	resolved, err := s.getUsersByIDs(ctx, misses)
	if err != nil {
		// Auth unavailable: report every miss as unresolved rather than failing.
		xlog.Error(ctx, "resolve user summaries failed, degrading to unknown", xfield.Error(err))
		resolved = nil
	}

	for _, id := range misses {
		user, ok := resolved[id]
		if !ok {
			out[id] = nil
			continue
		}
		s.cache.Set(id, user, ttlcache.DefaultTTL)
		out[id] = user
	}

	return out
}

// getUsersByIDs batch-resolves user profiles by id, indexed by id. Unlike the
// assignable-users path it does NOT exclude blocked users: a maintenance may
// have been authored by a since-blocked or removed user, and the read still
// wants to label them. Limit is bound to the id count — without it the store
// emits LIMIT 0 and resolves zero rows (ListUsers does not default Limit; the
// defaulting lives in the API binding layer this read path bypasses). Ids not
// present are simply absent from the map; the caller degrades them.
func (s *Service) getUsersByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*entity.User, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]*entity.User{}, nil
	}

	res, err := s.users.ListUsers(ctx, &entity.ListUsersCmd{
		IDs:   ids,
		Limit: int64(len(ids)),
	})
	if err != nil {
		return nil, fmt.Errorf("list users by ids: %w", err)
	}

	byID := make(map[uuid.UUID]*entity.User, len(res.Users))
	for _, u := range res.Users {
		byID[u.ID] = u
	}
	return byID, nil
}
