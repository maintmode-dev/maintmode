// Package license owns the SaaS license lifecycle on the instance side:
// collecting the heartbeat report (seat usage, last activity), talking to the
// Console license server, caching the returned license, and answering the
// block gate (the suspend middleware). The license is an
// optional feature: without it the process still starts and every consumer is
// wired with Noop instead — nothing is ever enforced.
package license

import (
	"cmp"
	"context"
	"time"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xcache"
)

// Enforcement is the license surface every consumer sees: the current license
// (the suspend-gate source). Implemented by Service (SaaS mode) and Noop
// (self-hosted) — callers never branch on "is the license configured".
type Enforcement interface {
	// License returns the current license, read through a short-TTL cache in
	// front of the shared license_cache row so every replica converges on the
	// same block state within the TTL. nil means nothing to enforce yet.
	License(ctx context.Context) *entity.License
}

// UsersStore lists the role sets of active (non-blocked) users. Defined
// consumer-side so the service can be tested with mocks.
type UsersStore interface {
	ListActiveRoles(ctx context.Context) ([][]entity.Role, error)
}

// InvitationsStore lists the role sets of live pending invitations.
type InvitationsStore interface {
	ListPendingRoles(ctx context.Context) ([][]entity.Role, error)
}

// AuditStore reports the newest domain-activity timestamp (nil = no activity).
type AuditStore interface {
	LastActivityAt(ctx context.Context) (*time.Time, error)
}

// Store persists the last successful heartbeat response.
type Store interface {
	Get(ctx context.Context) (*entity.License, error)
	Upsert(ctx context.Context, license *entity.License) error
}

// HeartbeatClient talks to the Console license server. Implemented by
// gateways/license.Client.
type HeartbeatClient interface {
	Send(ctx context.Context, report *entity.HeartbeatReport) (*entity.License, error)
}

type cacheKey string

// Service collects the heartbeat report, owns the cached license and answers
// the enforcement points. The heartbeat (Refresh) is the producer: it writes
// the license into the shared license_cache row. The enforcement reads
// (License) are the consumers: they read that row through a short-TTL
// process-local cache, so every replica converges on the same suspend/seat
// state within the TTL — the goque heartbeat tick is claimed by ONE replica,
// but all replicas see its result through the shared row.
type Service struct {
	users       UsersStore
	invitations InvitationsStore
	audit       AuditStore

	client  HeartbeatClient
	version string

	licenseStore Store

	cache *xcache.Cache[cacheKey, *entity.License]
}

// NewService builds the SaaS license service. cacheTTL bounds how stale a
// replica's view of the license may be (falls back to a minute); it is the
// read-through cache window in front of license_cache.
func NewService(
	users UsersStore,
	invitations InvitationsStore,
	audit AuditStore,
	store Store,
	client HeartbeatClient,
	version string,
	cacheTTL time.Duration,
) *Service {
	return &Service{
		users:        users,
		invitations:  invitations,
		audit:        audit,
		licenseStore: store,
		client:       client,
		version:      version,
		cache:        xcache.New[cacheKey, *entity.License](cmp.Or(cacheTTL, time.Minute)),
	}
}
