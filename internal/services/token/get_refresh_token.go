package token

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
)

// GetRefreshToken looks up a refresh token by its raw value (hashes internally).
func (s *Service) GetRefreshToken(ctx context.Context, rawToken string) (*entity.RefreshToken, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AccessToken.GetRefreshToken")
	defer span.End()

	return s.GetRefreshTokenByHash(ctx, xhash.HashSha256([]byte(rawToken)))
}

// GetRefreshTokenByHash looks up a refresh token by its hash directly.
func (s *Service) GetRefreshTokenByHash(ctx context.Context, hash string) (*entity.RefreshToken, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AccessToken.GetRefreshTokenByHash")
	defer span.End()

	token, err := s.tokensStore.GetByTokenHash(ctx, hash)
	if err != nil {
		xlog.Error(ctx, "failed to get refresh token", xfield.Error(err))
		return nil, err
	}

	return token, nil
}

// HasLiveRefreshToken reports whether a session still has a token that is
// neither revoked nor expired.
func (s *Service) HasLiveRefreshToken(ctx context.Context, family uuid.UUID) (bool, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AccessToken.HasLiveRefreshToken")
	defer span.End()

	live, err := s.tokensStore.HasLiveToken(ctx, family)
	if err != nil {
		xlog.Error(ctx, "failed to look up a live refresh token", xfield.Error(err))
		return false, err
	}

	return live, nil
}
