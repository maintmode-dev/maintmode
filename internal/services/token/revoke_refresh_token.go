package token

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
)

// RevokeRefreshTokenByFamily revokes all refresh tokens in the given family.
func (s *Service) RevokeRefreshTokenByFamily(ctx context.Context, family uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AccessToken.RevokeRefreshTokenByFamily")
	defer span.End()

	err := s.tokensStore.RevokeFamily(ctx, family)
	if err != nil {
		xlog.Error(ctx, "failed to revoke refresh token", xfield.Error(err))
		return err
	}

	s.revokeSessionAccessTokens(ctx, family)

	return nil
}

// RevokeRefreshTokenByUserID revokes all refresh tokens for the given user.
func (s *Service) RevokeRefreshTokenByUserID(ctx context.Context, userID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AccessToken.RevokeRefreshTokenByUserID")
	defer span.End()

	families, err := s.tokensStore.RevokeByUserID(ctx, userID)
	if err != nil {
		xlog.Error(ctx, "failed to revoke refresh token", xfield.Error(err))
		return err
	}

	s.revokeSessionAccessTokens(ctx, families...)

	return nil
}

// RevokeRefreshTokenByUserIDExceptFamily revokes every session of a user but
// one. See the store method for why the surviving session is the caller's own.
func (s *Service) RevokeRefreshTokenByUserIDExceptFamily(ctx context.Context, userID, keep uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AccessToken.RevokeRefreshTokenByUserIDExceptFamily")
	defer span.End()

	families, err := s.tokensStore.RevokeByUserIDExceptFamily(ctx, userID, keep)
	if err != nil {
		xlog.Error(ctx, "failed to revoke refresh tokens", xfield.Error(err))
		return err
	}

	s.revokeSessionAccessTokens(ctx, families...)

	return nil
}

// FamilyByRefreshToken resolves the session a raw refresh token belongs to, and
// verifies it belongs to the named user.
//
// This is how a caller names its OWN session: TokenPair.SessionID is never
// serialized, and the access token's sid is not trusted to pick a session to
// spare, so the refresh token is the handle a client proves it holds. The same
// shape /logout already uses.
func (s *Service) FamilyByRefreshToken(
	ctx context.Context,
	refreshTokenRaw string,
	userID uuid.UUID,
) (uuid.UUID, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AccessToken.FamilyByRefreshToken")
	defer span.End()

	rt, err := s.tokensStore.GetByTokenHash(ctx, xhash.HashSha256([]byte(refreshTokenRaw)))
	if err != nil {
		return uuid.Nil, err
	}

	// Ownership, not just validity: a token belonging to someone else must not
	// steer whose sessions are spared.
	if rt.UserID != userID {
		return uuid.Nil, apperr.ErrInvalidRefreshToken
	}

	// A revoked token names a dead family. Sparing it would revoke everything
	// except a session that is already gone, which is not what the caller asked
	// for -- they asked to keep the one they are using.
	if rt.Revoked {
		return uuid.Nil, apperr.ErrInvalidRefreshToken
	}

	return rt.Family, nil
}

// RevokeFamilyByRefreshToken revokes the whole session a raw refresh token
// belongs to, once the token is shown to belong to the access token's user, and
// returns that session.
//
// The whole family rather than the presented row: a predecessor rotated a
// moment earlier is still inside its grace window, and Refresh refuses it only
// when no token of its family is live any more.
func (s *Service) RevokeFamilyByRefreshToken(
	ctx context.Context, refreshTokenRaw string, accessClaims *entity.AccessClaims,
) (uuid.UUID, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AccessToken.RevokeFamilyByRefreshToken")
	defer span.End()

	var family uuid.UUID

	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		rt, err := s.tokensStore.GetByTokenHashForUpdate(ctx, xhash.HashSha256([]byte(refreshTokenRaw)))
		if err != nil {
			err := fmt.Errorf("%w: get refresh token: %w", apperr.ErrRefreshTokenNotFound, err)
			xlog.Error(ctx, "failed to get refresh token", xfield.Error(err))
			return err
		}

		// Verify ownership
		if rt.UserID.String() != accessClaims.Subject {
			err := fmt.Errorf("%w: mismatch user id in refresh token and access token", apperr.ErrInvalidAccessToken)
			xlog.Error(ctx, "mismatch user id in refresh token and access token",
				xfield.String("rt.user_id", rt.UserID.String()),
				xfield.String("access_claims.sub", accessClaims.Subject),
				xfield.Error(err),
			)
			return err
		}

		family = rt.Family

		return s.tokensStore.RevokeFamily(ctx, rt.Family)
	})
	if err != nil {
		xlog.Error(ctx, "failed to revoke refresh token", xfield.Error(err))
		return uuid.Nil, err
	}

	// After the commit, so a rolled-back revocation never cuts off a session
	// that is still alive.
	s.revokeSessionAccessTokens(ctx, family)

	return family, nil
}
