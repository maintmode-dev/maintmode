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
// identity. The password is resolved once at startup (see ResolvePassword) and
// held in memory: a generated one does not survive a restart, which is correct
// for a break-glass credential.
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

func (s *Service) MethodID() entity.AuthMethod {
	return entity.AuthMethodBootstrap
}
