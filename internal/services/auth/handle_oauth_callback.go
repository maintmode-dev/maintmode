package auth

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// HandleOAuthCallback exchanges the authorization code, creates or finds the user,
// and issues a token pair.
func (s *Service) HandleOAuthCallback(ctx context.Context, cmd *entity.HandleOAuthCallbackCmd) (*entity.TokenPair, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.HandleOAuthCallback")
	defer span.End()

	pair, user, err := s.handleOAuthCallback(ctx, cmd)
	if err != nil {
		s.auditorSrv.LogLogin(ctx, entity.AuditEventLoginFailed, user)
		xlog.Error(ctx, "failed to issue token pair", xfield.Error(err))
		return nil, fmt.Errorf("issue token pair: %w", err)
	}

	s.auditorSrv.LogLogin(ctx, entity.AuditEventLoginSuccess, user)

	return pair, nil
}

func (s *Service) handleOAuthCallback(ctx context.Context, cmd *entity.HandleOAuthCallbackCmd) (*entity.TokenPair, *entity.User, error) {
	providerUserInfo, err := s.getOAuthProviderUser(ctx, cmd.CallbackCode, cmd.Provider)
	if err != nil {
		xlog.Error(ctx, "failed to get oauth provider user", xfield.Error(err))
		return nil, nil, fmt.Errorf("get oauth provider user: %w", err)
	}

	user, err := s.usersSrv.GetOrCreateByOAuthInfo(ctx, providerUserInfo)
	if err != nil {
		xlog.Error(ctx, "failed to get or create user", xfield.Error(err))
		return nil, nil, fmt.Errorf("get or create user: %w", err)
	}

	pair, err := s.issueTokenPair(ctx, user, cmd.ClientIP)
	if err != nil {
		xlog.Error(ctx, "failed to issue token pair", xfield.Error(err))
		return nil, nil, fmt.Errorf("issue token pair: %w", err)
	}

	return pair, user, nil
}

func (s *Service) getOAuthProviderUser(ctx context.Context, code string, provider entity.OAuthProvider) (*entity.OAuthProviderUserInfo, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.getOAuthProviderUser")
	defer span.End()

	var userInfo *entity.OAuthProviderUserInfo
	switch provider {
	case entity.OAuthProviderGoogle:
		providerTokens, err := s.oauthProviders.Google.Exchange(ctx, code)
		if err != nil {
			return nil, fmt.Errorf("exchange code: %w", err)
		}

		userInfo, err = s.oauthProviders.Google.UserInfo(ctx, providerTokens.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("fetch user info: %w", err)
		}
	default:
		return nil, fmt.Errorf("%w: %s", apperr.ErrUnsupportedProvider, provider)
	}

	return userInfo, nil
}

func (s *Service) issueTokenPair(ctx context.Context, user *entity.User, clientIP string) (*entity.TokenPair, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.issueTokenPair")
	defer span.End()

	accessToken, err := s.tokenSrv.IssueAccessToken(ctx, accessTokenTTL, user)
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}

	raw, hashed, err := s.tokenSrv.GenerateRefreshToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	err = s.tokenSrv.SaveRefreshToken(ctx, &entity.RefreshToken{
		Token:     hashed,
		UserID:    user.ID,
		Family:    xuuid.New(),
		ExpiresAt: s.getNowF().Add(refreshTokenTTL),
		BoundIP:   clientIP,
	})
	if err != nil {
		return nil, fmt.Errorf("save refresh token: %w", err)
	}

	return &entity.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: raw,
		ExpiresIn:    int(accessTokenTTL.Seconds()),
	}, nil
}
