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

// LoginWithPasswordRequest is a sign-in with a password rather than an upstream
// token. Today it is served by the break-glass bootstrap admin; email_password
// joins it later on the same endpoint, which is why the shape already carries
// fields bootstrap does not use.
type LoginWithPasswordRequest struct {
	// Email is REQUIRED and must be a well-formed address of at most 254
	// characters, even though the bootstrap method ignores it when deciding who
	// signs in — that identity comes from configuration. It is required because
	// a failed attempt is attributed to it in the audit trail, and because the
	// later email_password method identifies by it.
	//
	// Saying so here matters more than usual: every failure of this endpoint
	// answers with the same opaque 401, so a client that omits the field gets
	// no runtime signal about why. This contract is the only place that can
	// tell an integrator the field is mandatory.
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
	// RememberMe is accepted and currently ignored: session modes are a separate
	// change. Present now so adding them later needs no wire-format change.
	RememberMe bool `json:"remember_me"`
}
