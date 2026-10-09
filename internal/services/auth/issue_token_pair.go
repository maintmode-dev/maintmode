package auth

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// IssueTokenPair mints the access/refresh pair for a freshly authenticated
// user and stamps the session's start, which every rotation in the chain then
// carries forward as the anchor for the maximum-lifetime limit. clientIP is the
// signing-in request's address, recorded on the session's first row.
func (s *Service) IssueTokenPair(ctx context.Context, user *entity.User, clientIP string) (*entity.TokenPair, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.IssueTokenPair")
	defer span.End()

	ttls := s.sessionTTLs(user)
	family := xuuid.New()

	accessToken, err := s.tokenSrv.IssueAccessToken(ctx, ttls.access, user, family)
	if err != nil {
		xlog.Error(ctx, "failed to issue access token", xfield.Error(err))
		return nil, fmt.Errorf("issue access token: %w", err)
	}

	raw, hashed, err := s.tokenSrv.GenerateRefreshToken(ctx)
	if err != nil {
		xlog.Error(ctx, "failed to generate refresh token", xfield.Error(err))
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	now := xtime.UTCNow()
	err = s.tokenSrv.SaveRefreshToken(ctx, &entity.RefreshToken{
		Token:            hashed,
		UserID:           user.ID,
		Family:           family,
		ExpiresAt:        now.Add(s.cfg.RefreshTokenTTL),
		ClientIP:         clientIP,
		SessionStartedAt: now,
	})
	if err != nil {
		xlog.Error(ctx, "failed to save refresh token", xfield.Error(err))
		return nil, fmt.Errorf("save refresh token: %w", err)
	}

	return &entity.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: raw,
		ExpiresIn:    int(ttls.access.Seconds()),
		SessionID:    family,
	}, nil
}
