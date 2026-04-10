package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
)

func (s *Service) Refresh(ctx context.Context, oldTokenRaw, clientIP string) (*entity.TokenPair, error) {
	now := s.getNowF()

	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.Refresh",
		xfield.Time("now", now),
	)
	defer span.End()

	// Hash the token once — used for DB lookup and as the lock key.
	tokenHash := xhash.HashSha256([]byte(oldTokenRaw))

	// Distributed lock prevents two concurrent refreshes of the same token
	// from both succeeding and creating duplicate token chains.
	lockKey := distributedLockKey(tokenHash)
	if err := s.locker.Acquire(ctx, lockKey, distributedLockTTL); err != nil {
		return nil, apperr.ErrLockBusy
	}
	defer func() {
		if err := s.locker.Release(ctx, lockKey); err != nil {
			xlog.Error(ctx, "failed to release lock", xfield.Error(err))
		}
	}()

	rt, err := s.tokenSrv.GetRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		return nil, apperr.ErrInvalidRefreshTokenToken
	}

	ctx = xlog.WithFields(ctx,
		xfield.Any("user", rt.UserID),
		xfield.Any("family", rt.Family),
		xfield.String("token_ip", rt.BoundIP),
		xfield.String("client_ip", clientIP),
	)

	// IP binding check — before any other logic.
	// Applies equally to revoked (grace period) and active tokens.
	if rt.BoundIP != clientIP {
		xlog.Error(ctx, "suspicious refresh")
		if err := s.tokenSrv.RevokeRefreshTokenByFamily(ctx, rt.Family); err != nil {
			xlog.Error(ctx, "failed to revoke family", xfield.Error(err))
		}
		return nil, apperr.ErrSuspiciousActivity
	}

	// Logout detection
	if rt.Revoked && rt.ReplacedBy == nil {
		return nil, apperr.ErrLogoutAlready
	}

	// Reuse detection — even if token is expired, reuse must revoke the family.
	if rt.Revoked {
		return s.reuseRevoked(ctx, now, rt)
	}

	return s.rotateRefreshToken(ctx, rt, now)
}

func (s *Service) reuseRevoked(ctx context.Context, now time.Time, rt *entity.RefreshToken) (*entity.TokenPair, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.buildTokenPairFromExisting")
	defer span.End()

	graceTTLExpiredAt := lo.FromPtr(rt.GraceTTL)
	if now.Before(graceTTLExpiredAt) {
		// Grace period

		// During grace period we cannot return the raw replacement token
		// because we only store hashes. The original Refresh call already
		// returned the raw token to the first caller. Subsequent callers
		// within the grace window get a fresh access token but must reuse
		// the refresh token they already have.
		rt, err := s.tokenSrv.GetRefreshTokenByHash(ctx, lo.FromPtr(rt.ReplacedBy))
		if err != nil {
			return nil, apperr.ErrRefreshTokenNotFound
		}

		accessToken, err := s.issueAccessByUserID(ctx, rt.UserID)
		if err != nil {
			return nil, err
		}

		return &entity.TokenPair{
			AccessToken:  accessToken,
			ExpiresIn:    int(accessTokenTTL.Seconds()),
			RefreshToken: "", // RefreshToken intentionally empty — client should keep its current one
		}, nil
	}

	// Outside grace period — reuse detection: revoke entire family.
	xlog.Error(ctx, "token reuse detected",
		xfield.String("grace_period", gracePeriod.String()),
		xfield.String("now", now.String()),
	)
	if err := s.tokenSrv.RevokeRefreshTokenByFamily(ctx, rt.Family); err != nil {
		xlog.Error(ctx, "failed to revoke family", xfield.Error(err))
	}
	return nil, fmt.Errorf("%w: refresh token's grace period has expired", apperr.ErrTokenReuse)
}

func (s *Service) rotateRefreshToken(ctx context.Context, oldRefreshToken *entity.RefreshToken, now time.Time) (*entity.TokenPair, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.issueNewRefreshToken")
	defer span.End()

	// Expiry check
	if now.After(oldRefreshToken.ExpiresAt) {
		xlog.Error(ctx, "refresh token expired", xfield.Time("expires_at", oldRefreshToken.ExpiresAt))
		return nil, apperr.ErrTokenExpired
	}

	// Issue access token before DB writes — if signing fails, no state is mutated.
	accessToken, err := s.issueAccessByUserID(ctx, oldRefreshToken.UserID)
	if err != nil {
		xlog.Error(ctx, "failed to issue access token", xfield.Error(err))
		return nil, err
	}

	// Normal rotation
	newRaw, newHash, err := s.tokenSrv.GenerateRefreshToken(ctx)
	if err != nil {
		xlog.Error(ctx, "failed to generate refresh token", xfield.Error(err))
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	// Atomic rotation: update old + save new in a single transaction.
	err = s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		oldRefreshToken.Revoked = true
		oldRefreshToken.GraceTTL = lo.ToPtr(now.Add(gracePeriod))
		oldRefreshToken.ReplacedBy = &newHash

		if err := s.tokenSrv.UpdateRefreshToken(txCtx, oldRefreshToken); err != nil {
			return fmt.Errorf("update old token: %w", err)
		}

		return s.tokenSrv.SaveRefreshToken(txCtx, &entity.RefreshToken{
			Token:     newHash,
			UserID:    oldRefreshToken.UserID,
			Family:    oldRefreshToken.Family,
			ExpiresAt: oldRefreshToken.ExpiresAt, // preserve original expiry
			BoundIP:   oldRefreshToken.BoundIP,
		})
	})
	if err != nil {
		xlog.Error(ctx, "failed to rotate refresh token", xfield.Error(err))
		return nil, err
	}

	return &entity.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: newRaw,
		ExpiresIn:    int(accessTokenTTL.Seconds()),
	}, nil
}

func (s *Service) issueAccessByUserID(ctx context.Context, userID uuid.UUID) (string, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.issueAccessByUserID")
	defer span.End()

	user, err := s.usersSrv.GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("fetch user: %w", err)
	}
	return s.tokenSrv.IssueAccessToken(ctx, accessTokenTTL, user)
}

func distributedLockKey(key string) string {
	return "refresh:" + key
}
