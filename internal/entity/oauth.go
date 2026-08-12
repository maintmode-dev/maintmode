package entity

type OAuthProvider string

const (
	// OAuthProviderStub keys the dev-only stub provider in the registry. It is
	// never accepted from a request: ParseOAuthProvider rejects it, and outside
	// dev the provider is not registered at all (RUK-249).
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

// ParseOAuthProvider validates s against the providers a client may name in a
// request and returns the typed provider. The bool is false for unknown values,
// and deliberately for OAuthProviderStub as well: its only caller reads the
// provider straight from the accept-invitation body.
func ParseOAuthProvider(s string) (OAuthProvider, bool) {
	switch OAuthProvider(s) {
	case OAuthProviderGoogle, OAuthProviderGithub:
		return OAuthProvider(s), true
	default:
		return "", false
	}
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
