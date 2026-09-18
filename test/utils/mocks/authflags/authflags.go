// Package authflags provides the shared test double for the built-in
// sign-in-method flags: every method offered, no errors.
//
// It lives here rather than in services/authsettings because that is what it
// is. The production code has exactly one flag source -- the settings service
// reading the table -- and a nil source refuses rather than allows (see the
// AuthMethodFlags doc in services/auth). What the tests need is the opposite
// answer for suites that are not about the flags at all, and a permissive
// double is a test fixture, not a production strategy.
//
// It is shared rather than hand-rolled per package because two packages need
// it -- services/auth and app/api/public/auth -- and a permissive double
// written twice is a permissive double that can disagree with itself. The
// precedent is mocks/publisher next door: a plain fixture for a capability
// several packages declare consumer-side.
package authflags

import (
	"context"

	"github.com/ruko1202/maintmode/internal/entity"
)

// AllEnabled answers every built-in method as offered.
//
// Satisfies both consumer-side interfaces without naming either: the one-method
// AuthMethodFlags that the sign-in gates read, and the List+Enabled pair the
// public listing reads.
type AllEnabled struct{}

func NewAllEnabled() AllEnabled { return AllEnabled{} }

// Enabled reports every method as offered, and never errors.
func (AllEnabled) Enabled(context.Context, entity.AuthMethodName) (bool, error) {
	return true, nil
}

// List reports every built-in as enabled, which is what the login-page listing
// reads. Synthesized rather than stored: there is no table behind this type,
// and the closed set is the only thing it could answer from.
func (AllEnabled) List(context.Context) ([]*entity.AuthMethodSetting, error) {
	names := entity.AllAuthMethodNames()

	out := make([]*entity.AuthMethodSetting, 0, len(names))
	for _, name := range names {
		out = append(out, &entity.AuthMethodSetting{Method: name, Enabled: true})
	}

	return out, nil
}
