package apiauthmodels

import (
	"fmt"

	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

// FromAPIConnectableProvider validates the :provider path parameter against the
// set of providers a user may connect or disconnect. The stub provider is an
// internal dev-only login backend and is intentionally not connectable.
func FromAPIConnectableProvider(s string) (entity.OAuthProvider, error) {
	provider := entity.OAuthProvider(s)
	switch provider {
	case entity.OAuthProviderGoogle, entity.OAuthProviderGithub:
		return provider, nil
	default:
		return "", fmt.Errorf("unsupported provider: %q", s)
	}
}

func ToAPITokenPairResponse(p *entity.TokenPair) *TokenPairResponse {
	return &TokenPairResponse{
		AccessToken:  p.AccessToken,
		RefreshToken: p.RefreshToken,
		ExpiresIn:    p.ExpiresIn,
	}
}

func ToAPIJWKSResponse(r entity.JWKS) *JWKSResponse {
	return &JWKSResponse{Keys: r.Keys}
}

func ToAPIIntrospectResponse(r *entity.IntrospectReport) *IntrospectResponse {
	return &IntrospectResponse{
		Active:  r.Active,
		JTI:     r.JTI,
		Subject: r.Subject,
		Email:   r.Email,
		Roles:   lo.Map(r.Roles, func(item entity.Role, _ int) string { return string(item) }),
		Exp:     r.Exp,
	}
}
