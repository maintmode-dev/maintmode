package entity

// AuthMethod names a way a user can authenticate. Today every method is an
// OAuth provider, but the vocabulary is deliberately wider: later work adds a
// password and an emailed code, neither of which is a provider.
//
// The string values are DATA, not just identifiers: they are written to and
// matched against user_identities.provider (a TEXT column with no CHECK
// constraint), and they reach the wire as the oauth_provider and
// connected_providers JSON fields. Changing a literal would not fail loudly —
// it would silently stop matching existing rows.
type AuthMethod string

const (
	// AuthMethodStub keys the dev-only stub provider in the registry. It is
	// never accepted from a request: authmethod.Methods.Parse rejects it, and
	// outside dev the provider is not registered at all.
	AuthMethodStub   AuthMethod = "stub"
	AuthMethodGoogle AuthMethod = "google"
	AuthMethodGithub AuthMethod = "github"
	// AuthMethodEmail has NO implementation behind it yet: nothing registers it,
	// so it is not in the vocabulary either. It exists here so the follow-up
	// work inherits a decided name instead of choosing one again.
	AuthMethodEmail AuthMethod = "email"
	// AuthMethodBootstrap keys the break-glass admin sign-in. Like the stub it is
	// never accepted from a request — Methods.Parse rejects it — but for the
	// opposite reason: the stub is refused because it verifies nothing, while
	// bootstrap is refused because it carries privileges no other method has (an
	// identity resolved by configured email, and an admin grant that skips the
	// seats cap). Those are safe only on the endpoint that gates them behind the
	// break-glass secret, so the method is reachable by that endpoint naming it
	// directly, never by a client naming it in a body.
	AuthMethodBootstrap AuthMethod = "bootstrap"
	// AuthMethodUnknown is an output-only sentinel for "no method known".
	// It is never a real login method and is never in the vocabulary.
	AuthMethodUnknown AuthMethod = "unknown"
)

// BootstrapSubject is the user_identities.subject of the break-glass admin.
//
// Every other method takes its subject from an upstream provider; bootstrap has
// no upstream, so the value is a constant. That constancy is what makes a
// repeat break-glass login resolve to the same user instead of creating a new
// one, and it is DATA in the same sense as the AuthMethod literals above: it is
// matched against existing user_identities rows, so changing it would silently
// orphan the admin identity rather than fail loudly.
const BootstrapSubject = "bootstrap"

// PrimaryAuthMethod returns the user's primary method — the first linked
// one — or AuthMethodUnknown when the list is empty. Used to populate the
// backward-compatible oauth_provider field, which predates connected_providers.
func PrimaryAuthMethod(methods []AuthMethod) AuthMethod {
	if len(methods) > 0 {
		return methods[0]
	}
	return AuthMethodUnknown
}

type OAuthProviderUserInfo struct {
	ID    string
	Email string
	Name  string
}

// OAuthIDTokenClaims is the verified subset of an upstream OIDC ID token that
// the backend trusts to identify a user.
type OAuthIDTokenClaims struct {
	Subject string
	Email   string
	Name    string
	// EmailVerified is the issuer's own answer to "have we checked that this
	// person controls this address". It governs whether Email may be used as an
	// identity key -- to match an invitation, or to claim a fresh account -- so
	// it travels with the address rather than being consumed and dropped where
	// the token is parsed.
	//
	// Providers with no upstream (the dev stub, break-glass) set it true: there
	// is no issuer to have checked, and the zero value would read as an issuer
	// reporting the address unverified.
	EmailVerified bool
}
