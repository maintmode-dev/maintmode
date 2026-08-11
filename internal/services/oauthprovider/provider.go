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

// OAuthProvider verifies identities asserted by an upstream OIDC provider.
//
// It deliberately does NOT do the authorization-code exchange: the BFF
// (maintmode-ui, NextAuth) owns the OAuth dance with Google and posts us the
// resulting id_token. The backend only verifies that token offline against the
// provider's JWKS, which is why it needs a client_id (the expected audience)
// but no client_secret.
type OAuthProvider interface {
	// ProviderID returns the provider's ID.
	ProviderID() entity.OAuthProvider
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
	providersMap[entity.OAuthProviderStub] = stuboauth.NewService()

	return &Providers{
		useStub:        cfg.Environment.IsDev() && cfg.OauthProviders.UseStub,
		providersStore: providersMap,
	}
}

func (p *Providers) Get(ctx context.Context, providerName entity.OAuthProvider) (OAuthProvider, error) {
	if p.useStub {
		xlog.Warn(ctx, "using stub oauth provider")
		providerName = entity.OAuthProviderStub
	}

	provider, ok := p.providersStore[providerName]
	if !ok {
		return nil, fmt.Errorf("%w: %s", apperr.ErrUnsupportedProvider, providerName)
	}

	return provider, nil
}
