package entity

import (
	"time"

	"github.com/google/uuid"
)

// LinkIntent records what a link ticket parks: whose account an identity is
// being attached to, and which provider the ticket was minted for.
//
// The provider is not redundant with the {provider} path segment -- it is what
// makes them checkable against each other. A ticket minted for one provider and
// presented on another's /start is refused, so a ticket cannot be diverted into
// linking an identity its owner never asked for.
type LinkIntent struct {
	UserID   uuid.UUID
	Provider AuthMethod
}

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
	// LinkTicket is echoed back for the oauth_link cookie, empty on an ordinary
	// sign-in. It travels unchanged rather than being re-parked behind a second
	// secret: it already points at a server-side entry, so the browser learns
	// nothing from holding it.
	LinkTicket string
}

// DanceOutcome is what a completed dance produced.
//
// Two fields rather than one string because "no code" is ambiguous with a bug:
// an empty Code could mean a link succeeded or that the sign-in path failed to
// mint one. The handler chooses between ?code= and ?linked=1, so it needs the
// intent stated rather than inferred.
type DanceOutcome struct {
	// Code is the one-time sign-in code, empty on a link.
	Code string
	// Linked reports that this dance attached an identity instead of signing a
	// user in. No token pair was minted; the person already had a session.
	Linked bool
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
	// LinkTicket arrives in the oauth_link cookie, empty on an ordinary sign-in.
	// Attacker-controlled like every field here -- a claim to be checked against
	// the store, never a fact. Its PRESENCE is the discriminator: once a browser
	// has presented one, no redeem result may produce a sign-in.
	LinkTicket string
	// InvitationHandle is the opaque handle /start planted for an invited dance,
	// empty for an ordinary one. Attacker-controlled like every field here: it
	// comes from a cookie, so it is a claim to be checked, never a fact. What
	// bounds it is that it is opaque and single-use -- redeeming it is the only
	// way to learn which invitation it names, and it names one at most once.
	InvitationHandle string
}
