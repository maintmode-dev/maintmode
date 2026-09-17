// Package oauthdance stores what a backend-driven OAuth dance cannot keep in the
// browser: the token pair waiting behind a one-time code, the invitation an
// invited dance is completing, and the account a link is being attached to.
//
// The state and the PKCE verifier are NOT here. They live in cookies, signed
// rather than stored, so no replica needs to know what another one issued —
// see the auth handlers. This entry exists only because the frontend is a
// different origin in production, so a cookie cannot carry the pair across.
//
// The invitation entry holds an invitation id behind an opaque handle, so the
// raw invitation token -- a bearer credential with a multi-day life -- never
// travels to the provider, into a redirect URL, or through a browser. The
// browser carries only the handle, in a cookie.
//
// The link entry is the same shape for a different job: it parks whose account
// an identity is being attached to, so a user id never travels through the
// provider's redirect. It shares the invitation entry's lifetime, because it has
// to survive the same consent screen.
//
// Everything here lives in Valkey and nothing in Postgres: both entries are
// worthless once their dance is over.
//
// Keys are the SHA-256 of the secret, never the secret itself. An operator
// running KEYS, a slow-log entry or a memory dump must not hand anyone a
// replayable credential.
package oauthdance

import (
	"time"

	valkeylib "github.com/redis/go-redis/v9"

	"github.com/ruko1202/maintmode/internal/utils/xhash"
)

const (
	codePrefix = "oauth:code:"
	// invitationPrefix is deliberately disjoint from codePrefix. The two entries
	// hold different things and live for different spans; a shared namespace
	// would let one be read as the other.
	invitationPrefix = "oauth:invitation:"
	// linkPrefix is disjoint from both for the same reason they are disjoint from
	// each other: these entries hold different things and a shared namespace
	// would let one be read as another.
	linkPrefix = "oauth:link:"
)

// Store is the Valkey-backed dance store.
//
// The TTL lives on the store rather than traveling per call: it is policy,
// identical for every dance, and a per-call value would be shared mutable state
// on a struct that concurrent requests share.
type Store struct {
	db      *valkeylib.Client
	codeTTL time.Duration
	// handleTTL covers both the invitation handle and the link ticket. One field
	// because both have to outlive the same consent screen, so they are one
	// policy rather than two that happen to match -- see NewStore.
	handleTTL time.Duration
}

// codeTTL bounds how long a one-time code is redeemable. Short because the code
// is a bearer credential with no second factor: whoever reads it inside the
// window can redeem it. Sixty seconds is the span between the browser receiving
// the redirect and the frontend exchanging it.
const codeTTL = 60 * time.Second

// NewStore creates an OAuth dance store.
//
// handleTTL is passed in rather than declared here because it must track the
// dance STATE lifetime, which is configurable (auth.oauth_dance_state_ttl): a
// handle has to outlive a consent screen with a password prompt and a second
// factor, exactly like the state does. A constant would silently decouple the
// two on any stand that tunes it, stranding invited sign-ins mid-consent. It is
// emphatically not codeTTL, which is a minute.
//
// The parameter keeps the invitation name at the call sites that already pass
// cfg.Auth.DanceStateTTL(); the field is named for what it now covers.
func NewStore(db *valkeylib.Client, invitationTTL time.Duration) *Store {
	// The link ticket reuses the same value rather than taking a parameter of
	// its own. Both have to outlive the same consent screen, and a second knob
	// of identical provenance would be four call-site edits for a policy nobody
	// can tune separately.
	return &Store{db: db, codeTTL: codeTTL, handleTTL: invitationTTL}
}

// codeKey is the ONE place the hashing scheme lives: both store methods address
// Valkey through it. The code is never a key itself, so a KEYS scan or a memory
// dump yields nothing redeemable.
func codeKey(code string) string { return codePrefix + xhash.HashSha256([]byte(code)) }

// invitationKey hashes the handle for the same reason codeKey hashes the code.
func invitationKey(handle string) string {
	return invitationPrefix + xhash.HashSha256([]byte(handle))
}

// linkKey hashes the ticket for the same reason again: the ticket attaches a
// sign-in method to an account, so a KEYS scan must not yield a usable one.
func linkKey(ticket string) string {
	return linkPrefix + xhash.HashSha256([]byte(ticket))
}
