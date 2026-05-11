package oauthprovider

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/oauthprovider/stuboauth"
)

type OAuthProvider interface {
	// ProviderID returns the provider's ID.'
	ProviderID() entity.OAuthProvider
	// AuthCodeURL returns the URL to redirect the user to for consent.
	// state is an opaque, already-encoded value (typically a SignedStateCodec output).
	AuthCodeURL(ctx context.Context, state string) string
	// Exchange trades an authorization code for Google tokensStore.
	Exchange(ctx context.Context, code string) (*entity.OAuthProviderTokens, error)
	// UserInfo fetches user profile from Google using the access token.
	UserInfo(ctx context.Context, accessToken string) (*entity.OAuthProviderUserInfo, error)
	// VerifyToken validates an ID token produced by an OIDC provider and returns the
	// trusted subset of claims used to identify a user.
	VerifyToken(ctx context.Context, idToken string) (*entity.OAuthIDTokenClaims, error)
}

type Providers struct {
	useStub        bool
	providersStore map[entity.OAuthProvider]OAuthProvider
}

func NewOAuthProviders(
	cfg *config.AppConfig,
	providers []OAuthProvider,
) *Providers {
	providersMap := lo.SliceToMap(providers, func(item OAuthProvider) (entity.OAuthProvider, OAuthProvider) {
		return item.ProviderID(), item
	})
	providersMap[entity.OAuthProviderStub] = stuboauth.NewService(&cfg.OauthProviders.Stub)

	p := &Providers{
		useStub:        cfg.Environment.IsDev() && cfg.OauthProviders.UseStub,
		providersStore: providersMap,
	}

	return p
}

func (p *Providers) Get(ctx context.Context, providerName entity.OAuthProvider) (OAuthProvider, error) {
	if p.useStub {
		xlog.Warn(ctx, "using stub oauth provider")
		stub, ok := p.providersStore[entity.OAuthProviderStub]
		if !ok {
			return nil, fmt.Errorf("stub oauth provider not found")
		}
		return stub, nil
	}

	provider, ok := p.providersStore[providerName]
	if !ok {
		return nil, fmt.Errorf("%w: %s", apperr.ErrUnsupportedProvider, provider)
	}

	return provider, nil
}
