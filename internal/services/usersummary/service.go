// Package usersummary resolves author/user summaries ({id, display_name, email})
// for read paths. The users table is owned by the auth module, so a maintenance
// author id is resolved by calling the auth user service in-process. Resolution
// degrades gracefully: when the lookup fails or the user no longer exists the
// resolver returns an id-only summary labeled "Unknown user" rather than failing
// the read.
//
// A short-lived in-memory cache (jellydator/ttlcache) fronts the user service so
// hot reads (and repeat authors across a list page) do not re-query it. The
// cache is TTL- and capacity-bounded, so it never grows without limit.
package usersummary

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"

	"github.com/ruko1202/maintmode/internal/entity"
)

const (
	// defaultTTL is how long a resolved user stays cached. Short by design:
	// author profiles (name/email) change rarely, and a minute bounds staleness
	// while still collapsing bursts of reads onto one user-service query.
	defaultTTL = time.Minute

	// defaultCapacity caps the number of cached users so the cache is
	// memory-bounded; least-recently-used entries are evicted past it.
	defaultCapacity = 10_000
)

// UserLister is the subset of the auth user service the resolver depends on.
// Declared consumer-side so tests can substitute a mock; satisfied by
// *user.Service. The batch-by-id composition (ListUsers without ExcludeBlocked
// so since-blocked authors still resolve, reindex to a map) lives in this
// package — see getUsersByIDs.
type UserLister interface {
	ListUsers(ctx context.Context, cmd *entity.ListUsersCmd) (*entity.ListUsersResult, error)
}

// Service resolves user summaries by id, caching resolved users in memory for a
// short TTL.
type Service struct {
	users UserLister
	cache *ttlcache.Cache[uuid.UUID, *entity.User]
}

// NewService builds the resolver with the default cache TTL and starts the
// cache's background eviction of expired entries. The janitor goroutine lives
// for the process lifetime, which matches the resolver (constructed once at
// startup).
func NewService(users UserLister) *Service {
	return newServiceWithTTL(users, defaultTTL)
}

// newServiceWithTTL builds the resolver with an explicit cache TTL. Exposed for
// tests that need a short TTL to exercise expiry without waiting a minute.
func newServiceWithTTL(users UserLister, ttl time.Duration) *Service {
	cache := ttlcache.New[uuid.UUID, *entity.User](
		ttlcache.WithTTL[uuid.UUID, *entity.User](ttl),
		ttlcache.WithCapacity[uuid.UUID, *entity.User](defaultCapacity),
	)
	go cache.Start()

	return &Service{
		users: users,
		cache: cache,
	}
}
