package auth

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// ExchangeIDToken verifies an upstream provider ID token and issues a backend
// token pair. This is the BFF-owned OAuth flow: the frontend completes the
// OAuth dance with the provider, receives the ID token, and posts it here so
// the backend can mint its own access/refresh pair.
func (s *Service) ExchangeIDToken(ctx context.Context, cmd *entity.ExchangeIDTokenCmd) (*entity.TokenPair, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.ExchangeIDToken")
	defer span.End()

	// Both the success and the failure records are published inside
	// SignInWithVerifiedClaims, which this path reaches through exchangeIDToken.
	// Cases that fail before identification (an invalid token) cannot be tied to
	// a user and are logged rather than audited.
	pair, _, err := s.exchangeIDToken(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "exchange id token failed", xfield.Error(err))
		return nil, err
	}

	return pair, nil
}

func (s *Service) exchangeIDToken(ctx context.Context, cmd *entity.ExchangeIDTokenCmd) (*entity.TokenPair, *entity.User, error) {
	authMethod, err := s.authMethods.Get(ctx, cmd.Provider)
	if err != nil {
		return nil, nil, fmt.Errorf("get oauth provider: %w", err)
	}

	claims, err := authMethod.Authenticate(ctx, cmd.IDToken)
	if err != nil {
		return nil, nil, err
	}

	// TestRoles are filled only by the dev component of the API layer; in prod
	// the field is always empty, so creation falls back to bootstrap/open-signup.
	// The policy is derived HERE rather than inside SignInWithVerifiedClaims,
	// because the other caller of that method — the backend OAuth dance — has no
	// X-Test-Roles header to derive it from and must pass the zero value.
	return s.SignInWithVerifiedClaims(ctx, cmd.Provider, claims, entity.UserCreationPolicy{
		AllowCreate: len(cmd.TestRoles) > 0,
		GrantRoles:  cmd.TestRoles,
	}, &entity.AuditMetadata{
		IP:        cmd.ClientIP,
		UserAgent: cmd.UserAgent,
	})
}
