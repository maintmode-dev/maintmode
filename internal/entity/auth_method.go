package entity

// AuthMethod names a way a user can authenticate. Today every method is an
// OAuth provider, but the vocabulary is deliberately wider: RUK-284 onward add
// a password and an emailed code, neither of which is a provider.
//
// The string values are DATA, not just identifiers: they are written to and
// matched against user_identities.provider (a TEXT column with no CHECK
// constraint), and they reach the wire as the oauth_provider and
// connected_providers JSON fields. Changing a literal would not fail loudly —
// it would silently stop matching existing rows. auth_method_test.go pins them.
type AuthMethod string

const (
	// AuthMethodStub keys the dev-only stub provider in the registry. It is
	// never accepted from a request: ParseAuthMethod rejects it, and outside
	// dev the provider is not registered at all (RUK-249).
	AuthMethodStub   AuthMethod = "stub"
	AuthMethodGoogle AuthMethod = "google"
	AuthMethodGithub AuthMethod = "github"
	// AuthMethodEmail and AuthMethodBootstrap are accepted by ParseAuthMethod
	// but have NO implementation behind them yet: the registry has no entry, so
	// Methods.Get refuses them. They exist here so tasks 2/13 onward inherit a
	// decided vocabulary instead of renaming it again (RUK-283).
	AuthMethodEmail     AuthMethod = "email"
	AuthMethodBootstrap AuthMethod = "bootstrap"
	// AuthMethodUnknown is an output-only sentinel for "no method known".
	// It is never a real login method and is rejected by ParseAuthMethod.
	AuthMethodUnknown AuthMethod = "unknown"
)

// PrimaryAuthMethod returns the user's primary method — the first linked
// one — or AuthMethodUnknown when the list is empty. Used to populate the
// backward-compatible oauth_provider field, which predates connected_providers.
func PrimaryAuthMethod(methods []AuthMethod) AuthMethod {
	if len(methods) > 0 {
		return methods[0]
	}
	return AuthMethodUnknown
}

// ParseAuthMethod validates s against the methods a client may name in a
// request and returns the typed method. The bool is false for unknown values,
// and deliberately for AuthMethodStub as well: its only caller reads the
// method straight from the accept-invitation body.
//
// Matching is exact — no case folding. A folded match would let "STUB" smuggle
// the stub provider back past this gate (RUK-249).
func ParseAuthMethod(s string) (AuthMethod, bool) {
	switch AuthMethod(s) {
	case AuthMethodGoogle, AuthMethodGithub, AuthMethodEmail, AuthMethodBootstrap:
		return AuthMethod(s), true
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
