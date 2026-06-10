package auth

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// HandleOAuthCallback exchanges the authorization code, creates or finds the user,
// and issues a token pair.
func (s *Service) HandleOAuthCallback(ctx context.Context, cmd *entity.HandleOAuthCallbackCmd) (*entity.TokenPair, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.HandleOAuthCallback")
	defer span.End()

	pair, user, err := s.handleOAuthCallback(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "failed to issue token pair", xfield.Error(err))
		return nil, fmt.Errorf("issue token pair: %w", err)
	}

	s.auditorSrv.LogLogin(ctx, entity.AuditEventLoginSuccess, user, &entity.AuditMetadata{
		IP:        cmd.ClientIP,
		UserAgent: cmd.UserAgent,
		SessionID: pair.SessionID.String(),
	})

	return pair, nil
}

func (s *Service) handleOAuthCallback(ctx context.Context, cmd *entity.HandleOAuthCallbackCmd) (*entity.TokenPair, *entity.User, error) {
	providerUserInfo, err := s.getOAuthProviderUser(ctx, cmd.CallbackCode, cmd.Provider)
	if err != nil {
		xlog.Error(ctx, "failed to get oauth provider user", xfield.Error(err))
		return nil, nil, fmt.Errorf("get oauth provider user: %w", err)
	}

	user, err := s.usersSrv.GetOrCreateByOAuthInfo(ctx, cmd.Provider, providerUserInfo)
	if err != nil {
		s.auditorSrv.LogLogin(ctx, entity.AuditEventLoginFailed, &entity.User{
			Email: providerUserInfo.Email,
			Name:  providerUserInfo.Name,
		}, &entity.AuditMetadata{
			IP:            cmd.ClientIP,
			UserAgent:     cmd.UserAgent,
			FailureReason: auditFailureUserProvisioning,
		})

		xlog.Error(ctx, "failed to get or create user", xfield.Error(err))
		return nil, nil, fmt.Errorf("get or create user: %w", err)
	}

	pair, err := s.IssueTokenPair(ctx, user, cmd.ClientIP)
	if err != nil {
		s.auditorSrv.LogLogin(ctx, entity.AuditEventLoginFailed, user, &entity.AuditMetadata{
			IP:            cmd.ClientIP,
			UserAgent:     cmd.UserAgent,
			FailureReason: auditFailureTokenIssuance,
		})

		xlog.Error(ctx, "failed to issue token pair", xfield.Error(err))
		return nil, nil, fmt.Errorf("issue token pair: %w", err)
	}

	return pair, user, nil
}

func (s *Service) getOAuthProviderUser(ctx context.Context, code string, provider entity.OAuthProvider) (*entity.OAuthProviderUserInfo, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.getOAuthProviderUser")
	defer span.End()

	oauthProvider, err := s.oauthProviders.Get(ctx, provider)
	if err != nil {
		return nil, fmt.Errorf("get oauth provider: %w", err)
	}

	providerTokens, err := oauthProvider.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	userInfo, err := oauthProvider.UserInfo(ctx, providerTokens.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("fetch user info: %w", err)
	}

	return userInfo, nil
}
