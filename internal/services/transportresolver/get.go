package transportresolver

import (
	"context"
	"fmt"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/gateways/notifytransport"
)

// Get implements notifytransport.TransportResolver. A missing/disabled
// integration — or a transport that has no builder — returns
// ErrIntegrationDisabled, which the dispatch path treats as a best-effort drop.
func (s *Service) Get(ctx context.Context, name entity.NotifyTransport) (notifytransport.Transport, error) {
	if t, ok := s.cache.Get(name); ok {
		return t, nil
	}

	// Snapshot the invalidation generation before reading the source, so a write
	// that commits + invalidates while we build below cannot be clobbered by our
	// set.
	gen := s.cache.Generation(name)

	build, ok := s.builders[name]
	if !ok {
		return nil, fmt.Errorf("%w: transport %q has no builder", apperr.ErrIntegrationDisabled, name)
	}

	// Domain junction: a channel's transport name doubles as the integration
	// kind (messenger_channels.transport == integration_settings.kind). This is
	// the one read-path point where the two string domains meet; a
	// delivery-capable kind takes its name from entity.NotifyTransport by
	// construction (see internal/integrationkinds), and the kind<->builder wiring is
	// pinned by TestBuilders_AlignWithIntegrationKinds.
	settings, err := s.source.Settings(ctx, string(name))
	if err != nil {
		return nil, err
	}

	transport, err := build(settings)
	if err != nil {
		return nil, fmt.Errorf("resolve %q: build transport: %w", name, err)
	}

	// Cache under the pre-read generation snapshot: if a write invalidated the
	// transport while we were building, Set is a no-op and this resolve returns
	// the freshly built (correct-for-now) transport without poisoning the cache
	// with stale state. Two concurrent cold resolves may both build (last valid
	// set wins); singleflight would be over-engineering at this cardinality.
	s.cache.Set(name, transport, gen)
	return transport, nil
}
