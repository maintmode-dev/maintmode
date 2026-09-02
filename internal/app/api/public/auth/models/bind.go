package apiauthmodels

import (
	"fmt"

	"github.com/ruko1202/maintmode/internal/entity"
)

// FromAPIConnectableProvider validates the :provider path parameter against the
// set of providers a user may connect or disconnect. The stub provider is an
// internal dev-only login backend and is intentionally not connectable.
func FromAPIConnectableProvider(s string) (entity.AuthMethod, error) {
	provider := entity.AuthMethod(s)
	switch provider {
	case entity.AuthMethodGoogle, entity.AuthMethodGithub:
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
