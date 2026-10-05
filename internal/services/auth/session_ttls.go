package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/entity"
)

// A break-glass session is short instead of being revoked when the password
// changes: refreshing does not carry it past a day after the sign-in.
const (
	breakGlassAccessTokenTTL  = 5 * time.Minute
	breakGlassSessionLifetime = 24 * time.Hour
)

// sessionTTLs are the access token's lifetime and the session's, from sign-in.
type sessionTTLs struct {
	access      time.Duration
	maxLifetime time.Duration
}

// sessionTTLs returns the configured lifetimes, or the break-glass ones --
// never longer than the configured -- for the break-glass account.
func (s *Service) sessionTTLs(user *entity.User) sessionTTLs {
	ttls := sessionTTLs{
		access:      s.cfg.AccessTokenTTL,
		maxLifetime: s.cfg.SessionMaxLifetime,
	}
	if !user.IsBreakGlass() {
		return ttls
	}

	return sessionTTLs{
		access:      min(ttls.access, breakGlassAccessTokenTTL),
		maxLifetime: min(ttls.maxLifetime, breakGlassSessionLifetime),
	}
}

func (s *Service) sessionTTLsFor(ctx context.Context, userID uuid.UUID) (sessionTTLs, error) {
	user, err := s.usersSrv.GetByID(ctx, userID)
	if err != nil {
		return sessionTTLs{}, fmt.Errorf("fetch user: %w", err)
	}

	return s.sessionTTLs(user), nil
}
