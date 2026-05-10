package entity

type OAuthProvider string

const (
	OAuthProviderGoogle OAuthProvider = "google"
)

type OAuthState struct {
	Nonce       string `json:"nonce"`                  // Nonce for CSRF (bound to nonce cookie).
	OriginalURI string `json:"original_uri,omitempty"` // OriginalURI is used to redirect for post-login navigation.
	ExpiresAt   int64  `json:"exp"`                    // ExpiresAt is the unix-seconds expiry; verified by SignedStateCodec.Decode.
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
