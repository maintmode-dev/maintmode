package stuboauth

import (
	"context"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
)

// Exchange trades an authorization code for Google tokens.
func (g *Service) Exchange(ctx context.Context, _ string) (*entity.OAuthProviderTokens, error) {
	_, span := xlog.WithOperationSpan(ctx, "service.OAuth.Stub.Exchange")
	defer span.End()

	return &entity.OAuthProviderTokens{
		AccessToken:  "access_token",
		RefreshToken: "refresh_token",
		IDToken:      "id_token",
	}, nil
}
