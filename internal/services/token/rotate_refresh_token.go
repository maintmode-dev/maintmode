package token

import (
	"context"
	"time"

	"github.com/ruko1202/xlog"
)

// RotateRefreshToken revokes a still-live refresh token in favor of its
// successor, keeping it usable until graceUntil. A token already revoked yields
// apperr.ErrRefreshTokenAlreadyRotated and is unchanged.
func (s *Service) RotateRefreshToken(ctx context.Context, tokenHash, successorHash string, graceUntil time.Time) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AccessToken.RotateRefreshToken")
	defer span.End()

	return s.tokensStore.Rotate(ctx, tokenHash, successorHash, graceUntil)
}
