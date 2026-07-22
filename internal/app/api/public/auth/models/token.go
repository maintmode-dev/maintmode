package apiauthmodels

import "github.com/ruko1202/maintmode/internal/entity"

type TokenPairResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
}

type JWKSResponse entity.JWKS

type ExchangeIDTokenRequest struct {
	// IDToken is the upstream provider's signed JWT.
	IDToken string `json:"id_token"`
}

// ConnectProviderRequest carries the upstream provider's signed JWT obtained by
// the frontend OAuth flow; the backend verifies it and links the identity to
// the authenticated user.
type ConnectProviderRequest struct {
	IDToken string `json:"id_token"`
}
