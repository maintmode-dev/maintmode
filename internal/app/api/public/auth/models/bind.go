package apiauthmodels

import (
	"github.com/ruko1202/maintmode/internal/entity"
)

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
