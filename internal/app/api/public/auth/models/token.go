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

// ConnectProviderRequest asks to attach an additional sign-in provider to the
// authenticated user, in one of two ways.
//
// Exactly one field must be set, and they are not interchangeable:
//
//   - IDToken is the BFF flow. The frontend ran the provider's dance itself and
//     posts the resulting ID token here.
//   - Mode "dance" asks the BACKEND to run the dance instead, and answers with a
//     link_url rather than linking anything on this request. It exists because a
//     provider with no id_token -- GitHub -- has nothing the frontend could post.
type ConnectProviderRequest struct {
	IDToken string `json:"id_token"`
	// Mode selects the backend-driven flow. The only accepted value is "dance";
	// anything else is refused rather than ignored, so a typo does not silently
	// fall back to the other branch.
	Mode string `json:"mode"`
}

// ConnectProviderDanceMode is the only value Mode accepts.
const ConnectProviderDanceMode = "dance"

// ConnectProviderDanceResponse answers a dance-mode connect.
//
// LinkURL is RELATIVE -- a path, with no origin. Nothing in configuration names
// this backend's own external base: frontend_url is the frontend, and a
// provider's redirect_uri is the callback. Deriving one by string surgery, or
// trusting the Host header, both break behind a proxy that strips a path
// prefix. The caller already knows the origin it just called, so it resolves the
// path against that.
//
// It must be followed by a TOP-LEVEL navigation, not fetch or XHR: the dance
// cookies are SameSite=Lax, so a background request drops the Set-Cookie and the
// dance dies as a 302 that reads as success in every access log.
type ConnectProviderDanceResponse struct {
	LinkURL string `json:"link_url"`
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
}

// ChangePasswordRequest sets the caller's own password.
//
// CurrentPassword is required when the account already has one and must be
// omitted when it does not -- the state right after a break-glass sign-in.
// Sending it in the wrong case is a 400 rather than a silently ignored field,
// so a client learns which state it is in.
//
// RefreshToken names the session to keep alive. It is optional: omitting it
// revokes every session, including the caller's, which is how an admin who has
// lost their refresh token can still set a password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password" binding:"required"`
	RefreshToken    string `json:"refresh_token"`
}
