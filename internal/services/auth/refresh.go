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
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
)

// Refresh rotates a refresh token and issues a new access token.
//
// The token is not bound to an address: a change is logged, never refused. A
// stolen token is caught by reuse detection instead.
func (s *Service) Refresh(ctx context.Context, oldTokenRaw, clientIP, userAgent string) (*entity.TokenPair, error) {
	now := xtime.UTCNow()

	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.Refresh",
		xfield.Time("now", now),
	)
	defer span.End()

	tokenHash := xhash.HashSha256([]byte(oldTokenRaw))

	// Two concurrent refreshes of one token must not both rotate it.
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

	// Revoked without a successor: the session was ended, not rotated.
	if rt.Revoked && rt.ReplacedBy == nil {
		return nil, apperr.ErrLogoutAlready
	}

	// A rotated token: a racing tab, or reuse -- checked even past expiry.
	if rt.Revoked {
		return s.reuseRevoked(ctx, now, rt, clientIP, userAgent)
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

// reuseRevoked answers a rotated token: inside its grace window a racing tab,
// past it reuse, which revokes the whole family.
func (s *Service) reuseRevoked(
	ctx context.Context, now time.Time, rt *entity.RefreshToken, clientIP, userAgent string,
) (*entity.TokenPair, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.reuseRevoked")
	defer span.End()

	if now.Before(lo.FromPtr(rt.GraceTTL)) {
		return s.refreshWithinGrace(ctx, rt)
	}

	xlog.Warn(ctx, "token reuse detected",
		xfield.String("grace_period", s.cfg.RefreshTokenGracePeriod.String()),
		xfield.String("now", now.String()),
	)
	if err := s.tokenSrv.RevokeRefreshTokenByFamily(ctx, rt.Family); err != nil {
		xlog.Error(ctx, "failed to revoke family", xfield.Error(err))
	}
	s.publishSessionRevoked(ctx, rt, clientIP, userAgent)
	return nil, fmt.Errorf("%w: refresh token's grace period has expired", apperr.ErrTokenReuse)
}

// refreshWithinGrace serves a second tab that refreshed a token a moment after
// the first rotated it -- the grace window exists because the lock does not
// wait. It gets an access token but keeps its refresh token: only hashes are
// stored, so the successor cannot be returned.
func (s *Service) refreshWithinGrace(ctx context.Context, rt *entity.RefreshToken) (*entity.TokenPair, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.refreshWithinGrace")
	defer span.End()

	live, err := s.tokenSrv.HasLiveRefreshToken(ctx, rt.Family)
	if err != nil {
		return nil, err
	}
	if !live {
		// Logout and reuse revoke the whole family: the session is over.
		return nil, apperr.ErrLogoutAlready
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
		RefreshToken: "", // the client keeps the one it has
	}, nil
}

// publishSessionRevoked records a reuse revocation against the session's owner.
// If the owner cannot be resolved the row is still written under the bare id:
// a thin actor beats a missing revocation.
func (s *Service) publishSessionRevoked(ctx context.Context, rt *entity.RefreshToken, clientIP, userAgent string) {
	owner, err := s.usersSrv.GetByID(ctx, rt.UserID)
	if err != nil {
		xlog.Error(ctx, "failed to resolve the owner of a revoked session", xfield.Error(err))
		owner = &entity.User{ID: rt.UserID}
	}

	s.publishAudit(ctx, audit.SessionRevoked{
		User: owner,
		Meta: &entity.AuditMetadata{
			IP:           clientIP,
			UserAgent:    userAgent,
			SessionID:    rt.Family.String(),
			RevokeReason: entity.AuditRevokeReasonTokenReuse,
		},
	})
}

func (s *Service) rotateRefreshToken(
	ctx context.Context, oldRefreshToken *entity.RefreshToken, now time.Time, clientIP string,
) (*entity.TokenPair, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.rotateRefreshToken")
	defer span.End()

	// Either limit ends the session. Both are read from the row's timestamps,
	// not a stored deadline, so lowering one affects existing sessions:
	// CreatedAt is the last rotation (idleness), SessionStartedAt the sign-in.
	ttls, err := s.sessionTTLsFor(ctx, oldRefreshToken.UserID)
	if err != nil {
		return nil, err
	}

	if idle := now.Sub(oldRefreshToken.CreatedAt); idle > s.cfg.SessionInactiveLifetime {
		xlog.Info(ctx, "session idle past its limit",
			xfield.String("idle_for", idle.String()),
			xfield.String("limit", s.cfg.SessionInactiveLifetime.String()),
		)
		return nil, apperr.ErrTokenExpired
	}
	if age := now.Sub(oldRefreshToken.SessionStartedAt); age > ttls.maxLifetime {
		xlog.Info(ctx, "session past its maximum lifetime",
			xfield.String("age", age.String()),
			xfield.String("limit", ttls.maxLifetime.String()),
		)
		return nil, apperr.ErrTokenExpired
	}

	// Signed before any DB write, so a signing failure mutates nothing.
	accessToken, err := s.issueAccessTokenByUserID(ctx, oldRefreshToken.UserID, ttls.access)
	if err != nil {
		xlog.Error(ctx, "failed to issue access token", xfield.Error(err))
		return nil, err
	}

	newRaw, newHash, err := s.tokenSrv.GenerateRefreshToken(ctx)
	if err != nil {
		xlog.Error(ctx, "failed to generate refresh token", xfield.Error(err))
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

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
			// Carried, never restarted: restarting it on every refresh would
			// make the maximum lifetime no maximum at all.
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
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.issueAccessTokenByUserID")
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
