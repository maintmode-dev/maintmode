// Package integration owns the registry of external-system connections
// (Slack, Telegram, SMTP, ...) whose config and secrets live in the DB and are
// managed by an admin at runtime (RUK-196). This file defines the per-kind
// contract and the static registry that maps a kind to its implementation, so
// adding a new integration type is one registration plus a small
// parser/validator/transport-builder — no schema change.
package integration

import (
	"fmt"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
)

// Registry maps a kind to its Integration. It is built once at startup and read
// concurrently thereafter, so it is not mutated after construction.
type Registry struct {
	byKind map[string]integrationkinds.Integration
}

// NewRegistry builds the registry from the given integrations, rejecting a
// duplicate kind so a misconfiguration fails fast at startup rather than silently
// shadowing one implementation with another.
func NewRegistry(integrations ...integrationkinds.Integration) (*Registry, error) {
	byKind := make(map[string]integrationkinds.Integration, len(integrations))
	for _, in := range integrations {
		kind := in.Kind()
		if kind == "" {
			return nil, fmt.Errorf("integration registry: empty kind")
		}
		if _, dup := byKind[kind]; dup {
			return nil, fmt.Errorf("integration registry: duplicate kind %q", kind)
		}
		byKind[kind] = in
	}
	return &Registry{byKind: byKind}, nil
}

// Get returns the Integration for a kind, or ErrUnknownIntegrationKind if no
// implementation is registered for it.
func (r *Registry) Get(kind string) (integrationkinds.Integration, error) {
	in, ok := r.byKind[kind]
	if !ok {
		return nil, fmt.Errorf("%w: %q", apperr.ErrUnknownIntegrationKind, kind)
	}
	return in, nil
}
