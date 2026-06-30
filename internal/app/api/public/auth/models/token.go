package apiauthmodels

import "github.com/ruko1202/maintmode/internal/entity"

type TokenPairResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
}

// OAuthCallbackJSONResponse is the structured response returned from the
// OAuth callback endpoint when the client requests JSON via Accept header.
// Refresh token is delivered in the body so server-side BFFs can persist it
// in their own session storage instead of relying on a shared cookie domain.
type OAuthCallbackJSONResponse struct {
	Token       TokenPairResponse `json:"token"`
	OriginalURI string            `json:"original_uri,omitempty"`
}

func ToOAuthCallbackJSONResponse(pair *entity.TokenPair, originalURI string) *OAuthCallbackJSONResponse {
	return &OAuthCallbackJSONResponse{
		Token:       *ToAPITokenPairResponse(pair),
		OriginalURI: originalURI,
	}
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
