// Package usersummary resolves author/user summaries ({id, display_name, email})
// for read paths. The users table is owned by the auth service, so a maintenance
// author id cannot be resolved locally — it is fetched from auth over S2S via the
// auth gateway. Resolution degrades gracefully: when auth is unavailable or the
// user no longer exists the resolver returns an id-only summary labeled
// "Unknown user" rather than failing the read.
//
// A short-lived in-memory cache (jellydator/ttlcache) fronts the gateway so hot
// reads (and repeat authors across a list page) do not hammer auth. The cache is
// TTL- and capacity-bounded, so it never grows without limit.
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
	// while still collapsing bursts of reads onto one auth call.
	defaultTTL = time.Minute

	// defaultCapacity caps the number of cached users so the cache is
	// memory-bounded; least-recently-used entries are evicted past it.
	defaultCapacity = 10_000
)

// AuthUsersGateway is the subset of the auth gateway the resolver depends on —
// batch user resolution by id. Defined consumer-side so tests can substitute a
// mock (mirrors userpicker.ActiveUsersLister).
type AuthUsersGateway interface {
	GetUsersByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*entity.User, error)
}

// Service resolves user summaries by id, caching resolved users in memory for a
// short TTL.
type Service struct {
	gateway AuthUsersGateway
	cache   *ttlcache.Cache[uuid.UUID, *entity.User]
}

// NewService builds the resolver with the default cache TTL and starts the
// cache's background eviction of expired entries. The janitor goroutine lives
// for the process lifetime, which matches the resolver (constructed once at
// startup).
func NewService(gateway AuthUsersGateway) *Service {
	return newServiceWithTTL(gateway, defaultTTL)
}

// newServiceWithTTL builds the resolver with an explicit cache TTL. Exposed for
// tests that need a short TTL to exercise expiry without waiting a minute.
func newServiceWithTTL(gateway AuthUsersGateway, ttl time.Duration) *Service {
	cache := ttlcache.New[uuid.UUID, *entity.User](
		ttlcache.WithTTL[uuid.UUID, *entity.User](ttl),
		ttlcache.WithCapacity[uuid.UUID, *entity.User](defaultCapacity),
	)
	go cache.Start()

	return &Service{
		gateway: gateway,
		cache:   cache,
	}
}
