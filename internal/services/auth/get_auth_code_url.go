package auth

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// GetAuthCodeURL returns Google consent URL for the given state parameter.
func (s *Service) GetAuthCodeURL(ctx context.Context, cmd *entity.GetAuthCodeURLCmd) (string, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.GetAuthCodeURL")
	defer span.End()

	switch cmd.Provider {
	case entity.OAuthProviderGoogle:
		return s.oauthProviders.Google.AuthCodeURL(ctx, cmd.State), nil
	default:
		return "", fmt.Errorf("%w: %s", apperr.ErrUnsupportedProvider, cmd.Provider)
	}
}
