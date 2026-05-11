package auth

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// GetAuthCodeURL returns Google consent URL for the given state parameter.
func (s *Service) GetAuthCodeURL(ctx context.Context, cmd *entity.GetAuthCodeURLCmd) (string, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auth.GetAuthCodeURL")
	defer span.End()

	oauthProvider, err := s.oauthProviders.Get(ctx, cmd.Provider)
	if err != nil {
		xlog.Error(ctx, "failed to get oauth provider", xfield.Error(err))
		return "", err
	}

	return oauthProvider.AuthCodeURL(ctx, cmd.State), nil
}
