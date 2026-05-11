package entity

type OAuthProvider string

const (
	OAuthProviderStub   OAuthProvider = "stub"
	OAuthProviderGoogle OAuthProvider = "google"
	OAuthProviderGithub OAuthProvider = "github"
)

type OAuthCallbackType = string

const (
	OAuthCallbackTypeHTML = "html"
	OAuthCallbackTypeJSON = "json"
)

func ToOAuthCallbackType(s string) OAuthCallbackType {
	switch s {
	case OAuthCallbackTypeHTML:
		return OAuthCallbackTypeHTML
	case OAuthCallbackTypeJSON:
		return OAuthCallbackTypeJSON
	default:
		return OAuthCallbackTypeJSON
	}
}

type OAuthState struct {
	Nonce             string            `json:"nonce"`                  // Nonce for CSRF (bound to nonce cookie).
	OriginalURI       string            `json:"original_uri,omitempty"` // OriginalURI is used to redirect for post-login navigation.
	OAuthCallbackType OAuthCallbackType `json:"oauth_callback_type"`    // OAuthCallbackType is used in the OauthCallback handler to determine the response type.
	ExpiresAt         int64             `json:"exp"`                    // ExpiresAt is the unix-seconds expiry; verified by SignedStateCodec.Decode.
}

type OAuthProviderTokens struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
}

type OAuthProviderUserInfo struct {
	ID    string
	Email string
	Name  string
}

// OAuthIDTokenClaims is the verified subset of an upstream OIDC ID token
// (currently Google) that the backend trusts to identify a user.
type OAuthIDTokenClaims struct {
	Subject string
	Email   string
	Name    string
}
