package auth

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/audit"

	"github.com/ruko1202/maintmode/internal/entity"
)

// ExchangeIDToken verifies an upstream provider ID token and issues a backend
// token pair. This is the BFF-owned OAuth flow: the frontend completes the
// OAuth dance with the provider, receives the ID token, and posts it here so
// the backend can mint its own access/refresh pair.
func (s *Service) ExchangeIDToken(ctx context.Context, cmd *entity.ExchangeIDTokenCmd) (*entity.TokenPair, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.ExchangeIDToken")
	defer span.End()

	pair, user, err := s.exchangeIDToken(ctx, cmd)
	if err != nil {
		// Login-failed audit is recorded inside exchangeIDToken once we
		// have a user identity; cases that fail before identification
		// (e.g. invalid token) cannot be tied to a user.
		xlog.Error(ctx, "exchange id token failed", xfield.Error(err))
		return nil, err
	}

	s.publishAudit(ctx, audit.LoginSuccess{
		User: user,
		Meta: &entity.AuditMetadata{
			IP:        cmd.ClientIP,
			UserAgent: cmd.UserAgent,
			SessionID: pair.SessionID.String(),
		},
	})
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
	user, err := s.usersSrv.GetOrCreateByAuthInfo(ctx, cmd.Provider, &entity.OAuthProviderUserInfo{
		ID:    claims.Subject,
		Email: claims.Email,
		Name:  claims.Name,
	}, entity.UserCreationPolicy{
		AllowCreate: len(cmd.TestRoles) > 0,
		GrantRoles:  cmd.TestRoles,
	})
	if err != nil {
		// login_failed is a security-relevant record. It is published to the
		// durable audit outbox: the write survives a crash, at the cost of being
		// eventually-consistent rather than persisted before we return.
		s.publishAudit(ctx, audit.LoginFailed{
			User: &entity.User{
				Email: claims.Email,
				Name:  claims.Name,
			},
			Meta: &entity.AuditMetadata{
				IP:            cmd.ClientIP,
				UserAgent:     cmd.UserAgent,
				FailureReason: provisioningFailureReason(err),
			},
		})
		return nil, nil, fmt.Errorf("get or create user: %w", err)
	}

	pair, err := s.IssueTokenPair(ctx, user, cmd.ClientIP)
	if err != nil {
		s.publishAudit(ctx, audit.LoginFailed{
			User: user,
			Meta: &entity.AuditMetadata{
				IP:            cmd.ClientIP,
				UserAgent:     cmd.UserAgent,
				FailureReason: entity.AuditFailureTokenIssuance,
			},
		})
		return nil, nil, fmt.Errorf("issue token pair: %w", err)
	}

	return pair, user, nil
}
