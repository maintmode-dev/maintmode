// Package integration owns the registry of external-system connections
// (Slack, Telegram, SMTP, ...) whose config and secrets live in the DB and are
// managed by an admin at runtime. This file defines the per-kind
// contract and the static registry that maps a kind to its implementation, so
// adding a new integration type is one registration plus a small
// parser/validator/transport-builder — no schema change.
package integration

import (
	"fmt"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
)

// Registry maps a SYSTEM NAME to its Integration. It is built once at startup
// and read concurrently thereafter, so it is not mutated after construction.
//
// The key is the name, not the category: "slack" and "google" each have an
// implementation, while "notify" and "login" have none. The set of registered
// names is what makes the name a closed set -- there is no second list to drift
// out of sync with it.
type Registry struct {
	byName map[string]integrationkinds.Integration
}

// NewRegistry builds the registry from the given integrations, rejecting a
// duplicate name so a misconfiguration fails fast at startup rather than silently
// shadowing one implementation with another.
func NewRegistry(integrations ...integrationkinds.Integration) (*Registry, error) {
	byName := make(map[string]integrationkinds.Integration, len(integrations))
	for _, in := range integrations {
		name := in.Name()
		if name == "" {
			return nil, fmt.Errorf("integration registry: empty name")
		}
		if _, dup := byName[name]; dup {
			return nil, fmt.Errorf("integration registry: duplicate name %q", name)
		}
		byName[name] = in
	}
	return &Registry{byName: byName}, nil
}

// Get returns the Integration for a system name, or ErrUnknownIntegrationKind
// if no implementation is registered under it.
func (r *Registry) get(name string) (integrationkinds.Integration, error) {
	in, ok := r.byName[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", apperr.ErrUnknownIntegrationKind, name)
	}
	return in, nil
}

// lookup returns the Integration registered under a system name, if any.
//
// The boolean sibling of get, for callers that treat "not registered" as a
// plain fact rather than a failure: the preset path asks about a name it has
// not admitted yet, and an unregistered one simply has no preset -- it is
// refused a moment later by admit, which is the guard that owns that decision.
func (r *Registry) lookup(name string) (integrationkinds.Integration, bool) {
	in, ok := r.byName[name]

	return in, ok
}

// admit refuses a (category, name) pair the registry does not hold.
//
// The verdict lives HERE rather than in a caller that asks which category a
// name belongs to and compares for itself. This is what makes the closed set a
// set of PAIRS rather than two independent lists: written as two lists --
// categories on one side, names on the other -- the set admits (notify,
// google), a row that walks past every login guard because they key on the
// category, past the preset rule because that keys on the name, and then
// reaches a delivery lookup that finds nothing.
//
// Two distinct errors, deliberately. An unregistered NAME is
// ErrUnknownIntegrationKind, the same sentinel the lookup raises, because it is
// the same condition and the API already maps it. A registered name under the
// WRONG category is ErrValidation naming the category it actually belongs to --
// collapsing the two would answer "no such integration" to an operator who sent
// a real one, and send them looking for a typo that is not there.
func (r *Registry) admit(category, name string) error {
	in, ok := r.byName[name]
	if !ok {
		return fmt.Errorf("%w: %q", apperr.ErrUnknownIntegrationKind, name)
	}

	// The entry answers for itself. The category travels with the
	// implementation rather than in a list beside it, so a new login provider
	// cannot be added and left out of the category by accident -- the compiler
	// refuses an Integration that does not say which half it belongs to.
	if registered := in.Category(); registered != category {
		return fmt.Errorf("%w: %q is a %s integration, not %s",
			apperr.ErrValidation, name, registered, category)
	}

	return nil
}
