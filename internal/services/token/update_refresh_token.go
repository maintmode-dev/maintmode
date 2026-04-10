package token

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// UpdateRefreshToken updates an existing refresh token (e.g. revoke, set grace TTL).
func (s *Service) UpdateRefreshToken(ctx context.Context, token *entity.RefreshToken) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AccessToken.UpdateRefreshToken")
	defer span.End()

	err := s.tokensStore.Update(ctx, token)
	if err != nil {
		xlog.Error(ctx, "failed to update refresh token", xfield.Error(err))
		return err
	}

	return nil
}
