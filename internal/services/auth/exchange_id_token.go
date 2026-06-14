package auth

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/eventbus/events"

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

	s.dispatcher.AsyncDispatch(ctx, events.UserLoginSuccess{
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
	oauthProvider, err := s.oauthProviders.Get(ctx, cmd.Provider)
	if err != nil {
		return nil, nil, fmt.Errorf("get oauth provider: %w", err)
	}

	claims, err := oauthProvider.VerifyToken(ctx, cmd.IDToken)
	if err != nil {
		return nil, nil, err
	}

	user, err := s.usersSrv.GetOrCreateByOAuthInfo(ctx, cmd.Provider, &entity.OAuthProviderUserInfo{
		ID:    claims.Subject,
		Email: claims.Email,
		Name:  claims.Name,
	})
	if err != nil {
		// login_failed is dispatched synchronously: a failed sign-in is a
		// security-relevant record that must be persisted before we return.
		// Dispatch is best-effort (listener errors are swallowed inside), so the
		// returned error cannot fail the request — ignore it deliberately.
		_ = s.dispatcher.Dispatch(ctx, events.UserLoginFailed{
			User: &entity.User{
				Email: claims.Email,
				Name:  claims.Name,
			},
			Meta: &entity.AuditMetadata{
				IP:            cmd.ClientIP,
				UserAgent:     cmd.UserAgent,
				FailureReason: entity.AuditFailureUserProvisioning,
			},
		})
		return nil, nil, fmt.Errorf("get or create user: %w", err)
	}

	pair, err := s.IssueTokenPair(ctx, user, cmd.ClientIP)
	if err != nil {
		_ = s.dispatcher.Dispatch(ctx, events.UserLoginFailed{
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
