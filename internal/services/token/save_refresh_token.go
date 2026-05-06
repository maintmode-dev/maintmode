package token

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// SaveRefreshToken persists a new refresh token.
func (s *Service) SaveRefreshToken(ctx context.Context, token *entity.RefreshToken) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AccessToken.SaveRefreshToken")
	defer span.End()

	err := s.tokensStore.Save(ctx, token)
	if err != nil {
		xlog.Error(ctx, "failed to save refresh token", xfield.Error(err))
		return fmt.Errorf("save refresh token: %w", err)
	}

	return nil
}
