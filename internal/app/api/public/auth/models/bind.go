package apiauthmodels

import (
	"github.com/samber/lo"

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
