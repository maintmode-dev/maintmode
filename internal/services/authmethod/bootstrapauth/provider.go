// Package bootstrapauth implements the break-glass admin sign-in: the
// emergency login that breaks the "to configure a provider you must sign in, to
// sign in you must configure a provider" loop on a fresh instance.
//
// Unlike every other AuthMethod it verifies a password rather than an upstream
// token, and its secret lives only in configuration — never in the database.
// It is registered in every environment, production included: gating it on
// "is the variable set" would leave a clean production instance with no way in
// at all, which is the problem this exists to solve. What makes it safe is the
// secret's entropy, the rate limiter in front of it, and an audit record of
// every attempt — not its absence.
package bootstrapauth

import (
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
)

// bootstrapUserName is the display name given to the break-glass admin when it
// is first created. A constant rather than a config key: it has exactly one
// consumer (users.name on creation) and no second one in sight.
const bootstrapUserName = "Bootstrap Admin"

// Service verifies the break-glass password and reports the configured admin
// identity. The password comes from configuration and is held in memory; an
// empty one means this instance has no break-glass, and Authenticate then
// refuses every candidate.
type Service struct {
	email    string
	password string
}

func NewService(cfg config.BootstrapConfig, password string) *Service {
	return &Service{
		email:    cfg.Email,
		password: password,
	}
}

// Email is the address the break-glass admin signs in as. The login path needs
// it to decide whether an incoming address is even a candidate for this method.
func (s *Service) Email() string {
	return s.email
}

func (s *Service) MethodID() entity.AuthMethod {
	return entity.AuthMethodBootstrap
}
