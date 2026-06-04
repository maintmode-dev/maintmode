package usersummary

import (
	"context"

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
// cache hits, fetches the misses from auth in one call, and labels any id that
// auth could not resolve (unavailable or unknown user) with the degraded
// "Unknown user" summary. The result contains an entry for every non-nil input
// id, so callers can map a list row's author without a nil check. It never
// returns an error — degrade, don't fail the read.
func (s *Service) ResolveMany(ctx context.Context, ids []uuid.UUID) map[uuid.UUID]*entity.UserSummary {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Usersummary.ResolveMany")
	defer span.End()

	uniqIDs := lo.Filter(lo.Uniq(ids), func(item uuid.UUID, _ int) bool {
		return item != uuid.Nil
	})

	out := make(map[uuid.UUID]*entity.UserSummary, len(uniqIDs))
	if len(uniqIDs) == 0 {
		return out
	}

	// Serve cache hits; collect the misses to fetch from auth in one call.
	misses := make([]uuid.UUID, 0, len(uniqIDs))
	for _, id := range uniqIDs {
		if item := s.cache.Get(id); item != nil {
			out[id] = item.Value().ToUserSummary()
			continue
		}
		misses = append(misses, id)
	}

	if len(misses) == 0 {
		return out
	}

	resolved, err := s.gateway.GetUsersByIDs(ctx, misses)
	if err != nil {
		// Auth unavailable: degrade every miss to the labeled fallback rather
		// than failing the read.
		xlog.Error(ctx, "resolve user summaries failed, degrading to unknown", xfield.Error(err))
		resolved = nil
	}

	for _, id := range misses {
		user, ok := resolved[id]
		if !ok {
			// Unknown user or auth down: label and move on (not cached — an id
			// may resolve once auth recovers or the user is recreated).
			xlog.Warn(ctx, "user summary unresolved, using unknown label", xfield.String("id", id.String()))
			out[id] = entity.NewUnknownUserSummary(id)
			continue
		}
		s.cache.Set(id, user, ttlcache.DefaultTTL)
		out[id] = user.ToUserSummary()
	}

	return out
}
