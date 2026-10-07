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
	// Binding is the browser binding the BFF presented, echoed back for the
	// oauth_binding cookie. See DanceCode.
	Binding string
}

// DanceCode is what a one-time sign-in code redeems: the pair, and the browser
// binding the dance began with.
//
// The binding is the hash of a nonce the BFF holds in the browser that started
// the dance. Without it a code is a bearer credential anyone can be handed: a
// member who stops their own dance at the redirect can send the code to a
// colleague and sign them into the sender's account. With it, only the BFF that
// holds the nonce can redeem the code.
type DanceCode struct {
	Pair    *TokenPair
	Binding string
}

// PendingLink is a link the provider has vouched for but the account owner has
// not yet confirmed. The callback parks it behind a one-time code; it becomes a
// linked identity only when the owner's own session redeems it.
//
// Completing at the callback is what let a ticket be planted: the callback is a
// browser navigation that proves nothing about who is driving the browser, so a
// victim sent someone else's link URL attached THEIR provider account to the
// SENDER's profile.
type PendingLink struct {
	UserID   uuid.UUID
	Provider AuthMethod
	Claims   OAuthIDTokenClaims
	Binding  string
}

// DanceOutcome is what a completed dance produced.
//
// Exactly one field is set. They are separate because they redeem at different
// endpoints and the handler sends them under different names (?code= and
// ?link_code=), so the kind has to be stated rather than inferred.
type DanceOutcome struct {
	// Code is the one-time sign-in code, empty on a link.
	Code string
	// LinkCode is the one-time code for a pending link, empty on a sign-in. No
	// token pair was minted; the person already has a session, and that session
	// is what redeems it.
	LinkCode string
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
	// Binding arrives in the oauth_binding cookie /start planted. It is carried
	// onto the one-time code, not checked here: only the BFF can prove it.
	Binding string
}

// CompleteLinkCmd redeems a pending link from the account owner's session.
type CompleteLinkCmd struct {
	// UserID is the session's user, from the access token.
	UserID       uuid.UUID
	LinkCode     string
	BindingProof string
	Meta         *AuditMetadata
}
