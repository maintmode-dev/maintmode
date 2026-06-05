package bootstrap

import (
	"fmt"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/gateways/notifytransport"
)

// AuthGateways contains all external gateways layer dependencies
type AuthGateways struct {
	NotifyTransportRegistry *notifytransport.Registry
}

// NewAuthGateways builds the gateways the auth binary needs. Unlike NewGateways
// it does NOT construct the auth S2S gateway — the auth service IS the auth
// service, so it has no external_services.auth to call — and only wires the
// notify-transport registry the invitation email outbox delivers through.
func NewAuthGateways(cfg *config.AppConfig) (*AuthGateways, error) {
	registry, err := notifyTransportRegistry(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init registry: %w", err)
	}

	return &AuthGateways{
		NotifyTransportRegistry: registry,
	}, nil
}
