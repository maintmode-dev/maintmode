package apiauthmodels

import "github.com/ruko1202/maintmode/internal/entity"

type TokenPairResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
}

type JWKSResponse entity.JWKS

type IntrospectRequest struct {
	AccessToken string `json:"access_token"`
}

type IntrospectResponse struct {
	Active  bool     `json:"active"`
	JTI     string   `json:"jti,omitempty"`
	Subject string   `json:"sub,omitempty"`
	Email   string   `json:"email,omitempty"`
	Roles   []string `json:"roles,omitempty"`
	Exp     int64    `json:"exp,omitempty"`
}
