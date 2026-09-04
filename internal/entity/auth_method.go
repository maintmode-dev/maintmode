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
	// never accepted from a request: ParseAuthMethod rejects it, and outside
	// dev the provider is not registered at all.
	AuthMethodStub   AuthMethod = "stub"
	AuthMethodGoogle AuthMethod = "google"
	AuthMethodGithub AuthMethod = "github"
	// AuthMethodEmail is accepted by ParseAuthMethod but has NO implementation
	// behind it yet: the registry has no entry, so Methods.Get refuses it. It
	// exists here so the follow-up work inherits a decided vocabulary instead of
	// renaming it again.
	AuthMethodEmail AuthMethod = "email"
	// AuthMethodBootstrap keys the break-glass admin sign-in. Like the stub it is
	// never accepted from a request — ParseAuthMethod rejects it — but for the
	// opposite reason: the stub is refused because it verifies nothing, while
	// bootstrap is refused because it carries privileges no other method has (an
	// identity resolved by configured email, and an admin grant that skips the
	// seats cap). Those are safe only on the endpoint that gates them behind the
	// break-glass secret, so the method is reachable by that endpoint naming it
	// directly, never by a client naming it in a body.
	AuthMethodBootstrap AuthMethod = "bootstrap"
	// AuthMethodUnknown is an output-only sentinel for "no method known".
	// It is never a real login method and is rejected by ParseAuthMethod.
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

// ParseAuthMethod validates s against the methods a client may name in a
// request and returns the typed method. The bool is false for unknown values,
// and deliberately for AuthMethodStub and AuthMethodBootstrap as well: its only
// caller reads the method straight from the accept-invitation body.
//
// Bootstrap is rejected for the same reason as the stub, and the reason is
// concrete rather than tidiness. Break-glass carries privileges no other method
// has — its identity resolves by configured email rather than by an upstream
// subject, and its admin grant deliberately skips the seats cap. Those
// privileges are safe only on the endpoint that gates them behind the
// break-glass secret. Letting a client NAME the method elsewhere would carry
// them onto a flow that never intended them: accepting an invitation with
// provider="bootstrap" would authenticate with the break-glass password and
// then take the privileged branches inside GetOrCreateByAuthInfo.
//
// Matching is exact — no case folding. A folded match would let "STUB" smuggle
// the stub provider back past this gate.
func ParseAuthMethod(s string) (AuthMethod, bool) {
	switch AuthMethod(s) {
	case AuthMethodGoogle, AuthMethodGithub, AuthMethodEmail:
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
