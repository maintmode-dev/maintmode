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

	return nil
}

// RevokeRefreshTokenByUserID revokes all refresh tokens for the given user.
func (s *Service) RevokeRefreshTokenByUserID(ctx context.Context, userID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AccessToken.RevokeRefreshTokenByUserID")
	defer span.End()

	err := s.tokensStore.RevokeByUserID(ctx, userID)
	if err != nil {
		xlog.Error(ctx, "failed to revoke refresh token", xfield.Error(err))
		return err
	}

	return nil
}

func (s *Service) RevokeRefreshToken(ctx context.Context, refreshTokenRaw string, accessClaims *entity.AccessClaims) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AccessToken.RevokeRefreshToken")
	defer span.End()

	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		rt, err := s.tokensStore.GetByTokenHashForUpdate(ctx, xhash.HashSha256([]byte(refreshTokenRaw)))
		if err != nil {
			return fmt.Errorf("%w: get refresh token: %w", apperr.ErrRefreshTokenNotFound, err)
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

		rt.Revoked = true

		return s.UpdateRefreshToken(ctx, rt)
	})
	if err != nil {
		xlog.Error(ctx, "failed to revoke refresh token", xfield.Error(err))
		return err
	}

	return nil
}
