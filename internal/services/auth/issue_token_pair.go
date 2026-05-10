package auth

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func (s *Service) IssueTokenPair(ctx context.Context, user *entity.User, clientIP string) (*entity.TokenPair, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.IssueTokenPair")
	defer span.End()

	accessToken, err := s.tokenSrv.IssueAccessToken(ctx, s.cfg.AccessTokenTTL, user)
	if err != nil {
		xlog.Error(ctx, "failed to issue access token", xfield.Error(err))
		return nil, fmt.Errorf("issue access token: %w", err)
	}

	raw, hashed, err := s.tokenSrv.GenerateRefreshToken(ctx)
	if err != nil {
		xlog.Error(ctx, "failed to generate refresh token", xfield.Error(err))
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	err = s.tokenSrv.SaveRefreshToken(ctx, &entity.RefreshToken{
		Token:     hashed,
		UserID:    user.ID,
		Family:    xuuid.New(),
		ExpiresAt: s.getNowF().Add(s.cfg.RefreshTokenTTL),
		BoundIP:   clientIP,
	})
	if err != nil {
		xlog.Error(ctx, "failed to save refresh token", xfield.Error(err))
		return nil, fmt.Errorf("save refresh token: %w", err)
	}

	return &entity.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: raw,
		ExpiresIn:    int(s.cfg.AccessTokenTTL.Seconds()),
	}, nil
}
