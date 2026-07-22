package entity

type OAuthProvider string

const (
	OAuthProviderStub   OAuthProvider = "stub"
	OAuthProviderGoogle OAuthProvider = "google"
	OAuthProviderGithub OAuthProvider = "github"
	// OAuthProviderUnknown is an output-only sentinel for "no provider known".
	// It is never a real login provider and is rejected by ParseOAuthProvider.
	OAuthProviderUnknown OAuthProvider = "unknown"
)

// PrimaryOAuthProvider returns the user's primary provider — the first linked
// one — or OAuthProviderUnknown when the list is empty. Used to populate the
// backward-compatible oauth_provider field, which predates connected_providers.
func PrimaryOAuthProvider(providers []OAuthProvider) OAuthProvider {
	if len(providers) > 0 {
		return providers[0]
	}
	return OAuthProviderUnknown
}

// ParseOAuthProvider validates s against the known providers and returns the
// typed provider. The bool is false for unknown values.
func ParseOAuthProvider(s string) (OAuthProvider, bool) {
	switch OAuthProvider(s) {
	case OAuthProviderStub, OAuthProviderGoogle, OAuthProviderGithub:
		return OAuthProvider(s), true
	default:
		return "", false
	}
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
