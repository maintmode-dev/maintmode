package oauthprovider

import (
	"context"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/oauthprovider/googleoauth"
	"github.com/ruko1202/maintmode/internal/services/oauthprovider/stuboauth"
)

type OAuthProvider interface {
	// AuthCodeURL returns the URL to redirect the user to for consent.
	AuthCodeURL(ctx context.Context, state *entity.OAuthState) string
	// Exchange trades an authorization code for Google tokensStore.
	Exchange(ctx context.Context, code string) (*entity.OAuthProviderTokens, error)
	// UserInfo fetches user profile from Google using the access token.
	UserInfo(ctx context.Context, accessToken string) (*entity.OAuthProviderUserInfo, error)
}

type Providers struct {
	Stub   OAuthProvider
	Google OAuthProvider
}

func NewOAuthProviders(
	env config.Environment,
	cfg *config.OauthProviders,
) *Providers {
	p := &Providers{
		Stub:   stuboauth.NewService(&cfg.Stub),
		Google: googleoauth.NewService(&cfg.Google),
	}

	if env.IsTest() {
		p.Google = p.Stub
	}

	return p
}
