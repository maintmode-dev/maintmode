package entity

import "time"

// DanceStart is what /start hands the browser.
type DanceStart struct {
	// State goes to the provider in the redirect, in the clear.
	State string
	// StateSignature goes to the browser as a cookie. The browser never sees
	// the state and the provider never sees the signature; a callback needs
	// both, which is the whole binding.
	StateSignature string
	// Verifier is the PKCE secret. It never travels to the provider — only its
	// S256 challenge does.
	Verifier string
	// AuthorizationURL is where the browser is sent, built so the client id,
	// redirect URI, scopes and challenge method come from one place.
	AuthorizationURL string
	// TTL is how long the signature stays valid, handed out so the transport can
	// match the cookies' MaxAge to it without knowing the number. It is a hint
	// to the browser either way: the enforced deadline is inside the signature.
	TTL time.Duration
	// InvitationHandle is empty unless the dance began from an invitation link.
	// It is opaque: it names an invitation only to whoever can redeem it against
	// the store, which happens once.
	InvitationHandle string
}

// DanceCallback is what a browser brings back from the provider.
//
// Every field is attacker-controlled: the query values come from the redirect,
// the cookie values from whatever the client chose to send.
type DanceCallback struct {
	// Provider is the {provider} path segment, unvalidated.
	Provider string
	// ProviderError is the provider reporting its own failure, if it did.
	ProviderError string
	// State and Code arrive in the query; StateSignature and Verifier in the
	// cookies /start planted.
	State          string
	Code           string
	StateSignature string
	Verifier       string
	// InvitationHandle is the opaque handle /start planted for an invited dance,
	// empty for an ordinary one. Attacker-controlled like every field here: it
	// comes from a cookie, so it is a claim to be checked, never a fact. What
	// bounds it is that it is opaque and single-use -- redeeming it is the only
	// way to learn which invitation it names, and it names one at most once.
	InvitationHandle string
}
