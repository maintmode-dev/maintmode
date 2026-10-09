package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
)

// Refresh rotates a refresh token and issues a new access token.
//
// The token is not bound to an address: a change is logged, never refused. A
// stolen token is caught by reuse detection instead.
func (s *Service) Refresh(ctx context.Context, oldTokenRaw, clientIP string) (*entity.TokenPair, error) {
	now := xtime.UTCNow()

	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.Refresh",
		xfield.Time("now", now),
	)
	defer span.End()

	// Hash the token once — used for DB lookup and as the lock key.
	tokenHash := xhash.HashSha256([]byte(oldTokenRaw))

	// Distributed lock prevents two concurrent refreshes of the same token
	// from both succeeding and creating duplicate token chains.
	lockKey := distributedLockKey(tokenHash)
	if err := s.locker.Acquire(ctx, lockKey, s.cfg.RefreshTokenrDistributedLockTTL); err != nil {
		return nil, apperr.ErrLockBusy
	}
	defer func() {
		if err := s.locker.Release(ctx, lockKey); err != nil {
			xlog.Error(ctx, "failed to release lock", xfield.Error(err))
		}
	}()

	rt, err := s.tokenSrv.GetRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		return nil, apperr.ErrInvalidRefreshToken
	}

	ctx = xlog.WithFields(ctx,
		xfield.Any("user", rt.UserID),
		xfield.Any("family", rt.Family),
	)

	// Logout detection
	if rt.Revoked && rt.ReplacedBy == nil {
		return nil, apperr.ErrLogoutAlready
	}

	// Reuse detection — even if token is expired, reuse must revoke the family.
	if rt.Revoked {
		return s.reuseRevoked(ctx, now, rt)
	}

	return s.rotateRefreshToken(ctx, rt, now, clientIP)
}

// logClientChange records a rotation from a new address. Info, not a warning:
// networks change under people; the line is for correlating with a later reuse
// revocation. The family comes from ctx.
func logClientChange(ctx context.Context, prev *entity.RefreshToken, clientIP string) {
	if prev.ClientIP == clientIP {
		return
	}

	xlog.Info(ctx, "session refreshed from a new address",
		xfield.String("prev_ip", prev.ClientIP),
		xfield.String("ip", clientIP),
	)
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
		rt, err := s.liveSuccessor(ctx, rt)
		if err != nil {
			return nil, err
		}

		ttls, err := s.sessionTTLsFor(ctx, rt.UserID)
		if err != nil {
			return nil, err
		}

		accessToken, err := s.issueAccessTokenByUserID(ctx, rt.UserID, ttls.access)
		if err != nil {
			return nil, err
		}

		return &entity.TokenPair{
			AccessToken:  accessToken,
			ExpiresIn:    int(ttls.access.Seconds()),
			RefreshToken: "", // RefreshToken intentionally empty — client should keep its current one
		}, nil
	}

	// Outside grace period — reuse detection: revoke entire family.
	xlog.Error(ctx, "token reuse detected",
		xfield.String("grace_period", s.cfg.RefreshTokenGracePeriod.String()),
		xfield.String("now", now.String()),
	)
	if err := s.tokenSrv.RevokeRefreshTokenByFamily(ctx, rt.Family); err != nil {
		xlog.Error(ctx, "failed to revoke family", xfield.Error(err))
	}
	return nil, fmt.Errorf("%w: refresh token's grace period has expired", apperr.ErrTokenReuse)
}

func (s *Service) rotateRefreshToken(
	ctx context.Context, oldRefreshToken *entity.RefreshToken, now time.Time, clientIP string,
) (*entity.TokenPair, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.issueNewRefreshToken")
	defer span.End()

	// Two independent limits, either of which ends the session. They are
	// evaluated against timestamps on the row rather than a deadline stored at
	// issue time, so lowering a limit takes effect on sessions that already
	// exist instead of only on new ones.
	//
	// CreatedAt is the row's own age, and rotation inserts a new row, so it
	// measures time since the last rotation -- i.e. idleness. SessionStartedAt
	// is carried unchanged down the chain and measures time since sign-in.
	ttls, err := s.sessionTTLsFor(ctx, oldRefreshToken.UserID)
	if err != nil {
		return nil, err
	}

	if idle := now.Sub(oldRefreshToken.CreatedAt); idle > s.cfg.SessionInactiveLifetime {
		xlog.Error(ctx, "session idle past its limit",
			xfield.String("idle_for", idle.String()),
			xfield.String("limit", s.cfg.SessionInactiveLifetime.String()),
		)
		return nil, apperr.ErrTokenExpired
	}
	if age := now.Sub(oldRefreshToken.SessionStartedAt); age > ttls.maxLifetime {
		xlog.Error(ctx, "session past its maximum lifetime",
			xfield.String("age", age.String()),
			xfield.String("limit", ttls.maxLifetime.String()),
		)
		return nil, apperr.ErrTokenExpired
	}

	// Issue access token before DB writes — if signing fails, no state is mutated.
	accessToken, err := s.issueAccessTokenByUserID(ctx, oldRefreshToken.UserID, ttls.access)
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
		oldRefreshToken.GraceTTL = lo.ToPtr(now.Add(s.cfg.RefreshTokenGracePeriod))
		oldRefreshToken.ReplacedBy = &newHash

		if err := s.tokenSrv.UpdateRefreshToken(txCtx, oldRefreshToken); err != nil {
			return fmt.Errorf("update old token: %w", err)
		}

		return s.tokenSrv.SaveRefreshToken(txCtx, &entity.RefreshToken{
			Token:     newHash,
			UserID:    oldRefreshToken.UserID,
			Family:    oldRefreshToken.Family,
			ExpiresAt: now.Add(s.cfg.RefreshTokenTTL),
			ClientIP:  clientIP,
			// Carried, never restarted: this is what bounds the session's total
			// age. Recomputing it here would reset the ceiling on every refresh,
			// quietly turning the maximum lifetime into no maximum at all.
			SessionStartedAt: oldRefreshToken.SessionStartedAt,
		})
	})
	if err != nil {
		xlog.Error(ctx, "failed to rotate refresh token", xfield.Error(err))
		return nil, err
	}

	logClientChange(ctx, oldRefreshToken, clientIP)

	return &entity.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: newRaw,
		ExpiresIn:    int(ttls.access.Seconds()),
	}, nil
}

func (s *Service) issueAccessTokenByUserID(ctx context.Context, userID uuid.UUID, ttl time.Duration) (string, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.issueAccessByUserID")
	defer span.End()

	user, err := s.usersSrv.GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("fetch user: %w", err)
	}
	return s.tokenSrv.IssueAccessToken(ctx, ttl, user)
}

func distributedLockKey(key string) string {
	return "refresh:" + key
}

// maxGraceHops bounds the walk down a rotation chain. A grace window is
// seconds long, so a chain more than a few rotations deep inside one is not a
// client racing itself, and it is refused rather than followed.
const maxGraceHops = 8

// liveSuccessor follows a revoked token's replacements to the end of its chain
// and returns that end only if the session is still alive there.
//
// The grace window answers a client that refreshed twice in one rotation, and
// it must not outlive the session: logout revokes only the CURRENT token, so
// without this walk a predecessor rotated a moment earlier kept minting access
// tokens through its window -- a stolen copy surviving the very logout meant to
// end it.
func (s *Service) liveSuccessor(ctx context.Context, rt *entity.RefreshToken) (*entity.RefreshToken, error) {
	for range maxGraceHops {
		next, err := s.tokenSrv.GetRefreshTokenByHash(ctx, lo.FromPtr(rt.ReplacedBy))
		if err != nil {
			return nil, apperr.ErrRefreshTokenNotFound
		}

		switch {
		case !next.Revoked:
			return next, nil
		case next.ReplacedBy == nil:
			// Revoked without a successor: the session was ended there.
			return nil, apperr.ErrLogoutAlready
		}

		rt = next
	}

	xlog.Warn(ctx, "rotation chain too deep for a grace refresh")

	return nil, apperr.ErrInvalidRefreshToken
}
