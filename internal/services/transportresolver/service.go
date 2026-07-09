// Package transportresolver is the delivery-side counterpart of the integration
// registry (grafana-shape split): the registry stores configs and secrets and
// knows nothing about transports; this service owns the transport -> builder
// mapping, builds clients from the registry's typed settings, and caches them.
// It is the production implementation of notifytransport.TransportResolver
// (bootstrap swaps in the stub resolver for dev use_stub).
package transportresolver

import (
	"context"
	"time"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/gateways/notifytransport"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
	"github.com/ruko1202/maintmode/internal/utils/xcache"
)

// cacheTTL bounds how long a built transport can be served from a replica that
// did not observe the write that changed it. A write on the serving replica
// invalidates synchronously via the integration service's onChange hook
// (immediate effect there); this TTL is the ceiling on staleness elsewhere.
// Kept short so "toggle off" takes effect promptly fleet-wide.
const cacheTTL = 30 * time.Second

// SettingsSource is the consumer-side view of the integration registry: the
// typed, decrypted settings of an enabled integration, looked up by kind.
// Implemented by services/integration; a disabled/unconfigured kind surfaces as
// apperr.ErrIntegrationDisabled.
type SettingsSource interface {
	// Settings yields the kind's parsed settings (whatever its Parse produced) —
	// consumed only by that kind's Builder below.
	Settings(ctx context.Context, kind string) (integrationkinds.Settings, error)
}

// Service resolves transports for delivery: settings from the registry, client
// construction via the per-transport builder, generation-guarded caching on top.
type Service struct {
	source   SettingsSource
	builders map[entity.NotifyTransport]Builder
	cache    *xcache.Cache[entity.NotifyTransport, notifytransport.Transport]
}

// The service is the live delivery resolver; bootstrap hands the sender and the
// async processor either this or the dev StubResolver.
var _ notifytransport.TransportResolver = (*Service)(nil)

// New builds the resolver over the given settings source and transport builders.
func New(source SettingsSource, builders map[entity.NotifyTransport]Builder) *Service {
	return &Service{
		source:   source,
		builders: builders,
		cache:    xcache.New[entity.NotifyTransport, notifytransport.Transport](cacheTTL),
	}
}

// Invalidate drops the cached transport built from kind's settings and bumps
// its generation, so an in-flight resolve cannot repopulate the cache with
// pre-write state. Wired as the integration service's onChange hook (which
// speaks the KIND domain — the write-path side of the same domain junction as
// Get's settings lookup); called synchronously after every committed write.
func (s *Service) Invalidate(kind string) {
	s.cache.Invalidate(entity.NotifyTransport(kind))
}
