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
	//
	// Where a password is configured it is PERMANENTLY LIVE, and that is a
	// decision rather than an omission.
	// Demoting it to a one-time seed -- spent when the admin gains a password of
	// their own -- was designed and rejected: recovery would still mean editing
	// the secrets file on the host (there is no environment override, by
	// design), so a denylist of spent values buys nothing that a config edit
	// does not already give, while costing a table whose rows are argon2id
	// hashes stored in order to be REJECTED, scanned rather than looked up
	// because the salt forbids an index.
	//
	// What makes a permanent credential acceptable is not its absence but its
	// bounds: the rate limiter in front of it, a repeat login granting no new
	// privileges (roles apply only on creation), revocation by blocking the
	// admin, and -- the part that was missing -- an audit record naming it as
	// the credential that answered. See entity.AuditLoginMethod.
	//
	// Where none is configured there is no break-glass at all: the provider
	// stays registered and refuses every candidate, so the instance signs people
	// in by the ordinary methods and the refusal is indistinguishable from a
	// wrong address.
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

// OAuth2Identity is what an OAuth 2.0 access token resolves to.
//
// The shape the auth method needs and nothing more: no access token, no vendor
// handle beyond the display name, no group or org list. What is absent is
// deliberate -- the token never leaves the request that obtained it, so it is
// not carried here where a caller could be tempted to persist it.
//
// It is NOT OAuthProviderUserInfo, whose ID is a provider's user id as the user
// service wants it. Subject here is the value written to
// user_identities.subject, and naming it so is what keeps the two from being
// assigned across without anyone noticing the meaning changed.
type OAuth2Identity struct {
	// Subject is the vendor's STABLE account key. Never a renameable handle:
	// a login, once renamed, is claimable by someone else, so an account keyed
	// on one could be inherited.
	Subject string
	// Email is the account's primary AND verified address. Any other address is
	// refused by the vendor -- this field never carries an unvouched one.
	Email string
	Name  string
}

// OAuth2Credentials is the OAuth app a gateway acts as, and where it talks.
//
// The endpoints travel WITH the credentials because a plain OAuth 2.0 provider
// publishes no discovery document: there is nothing to fetch them from, so they
// are configuration like the client id is. They reach a row from the deployment
// catalog rather than from an operator -- see config.LoginPreset -- which is
// what stops anyone pointing a known provider name at a host they control.
type OAuth2Credentials struct {
	// DisplayName labels log lines and spans. It is not part of the exchange.
	DisplayName string
	ClientID    string
	// ClientSecret authenticates the app at the token endpoint. Required for
	// every vendor: a plain OAuth 2.0 identity cannot be established without
	// spending it.
	ClientSecret string
	// RedirectURI must be the EXTERNAL callback URL, matching what is
	// registered at the provider.
	RedirectURI string
	// AuthorizeURL is where the browser is sent to consent, and TokenURL is
	// where the authorization code is exchanged. Both are the GRANT's, which is
	// why they are here: the grant is what this struct configures, and both go
	// straight into oauth2.Endpoint.
	//
	// The identity API's base is deliberately NOT here. It configures the reads
	// a vendor makes, not the grant, so it reaches the vendor directly -- see
	// the Vendor contract.
	AuthorizeURL string
	// TokenURL is where the client secret travels, which is why it is validated
	// as https upstream.
	TokenURL string
}
