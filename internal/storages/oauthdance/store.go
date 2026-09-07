// Package oauthdance stores the one thing a backend-driven OAuth dance cannot
// keep in the browser: the token pair waiting behind a one-time code.
//
// The state and the PKCE verifier are NOT here. They live in cookies, signed
// rather than stored, so no replica needs to know what another one issued —
// see the auth handlers. This entry exists only because the frontend is a
// different origin in production, so a cookie cannot carry the pair across.
//
// Everything here lives in Valkey and nothing in Postgres: the entry is
// worthless a minute after it is written.
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

const codePrefix = "oauth:code:"

// Store is the Valkey-backed dance store.
//
// The TTL lives on the store rather than traveling per call: it is policy,
// identical for every dance, and a per-call value would be shared mutable state
// on a struct that concurrent requests share.
type Store struct {
	db      *valkeylib.Client
	codeTTL time.Duration
}

// codeTTL bounds how long a one-time code is redeemable. Short because the code
// is a bearer credential with no second factor: whoever reads it inside the
// window can redeem it. Sixty seconds is the span between the browser receiving
// the redirect and the frontend exchanging it.
const codeTTL = 60 * time.Second

// NewStore creates an OAuth dance store.
func NewStore(db *valkeylib.Client) *Store {
	return &Store{db: db, codeTTL: codeTTL}
}

// codeKey is the ONE place the hashing scheme lives: both store methods address
// Valkey through it. The code is never a key itself, so a KEYS scan or a memory
// dump yields nothing redeemable.
func codeKey(code string) string { return codePrefix + xhash.HashSha256([]byte(code)) }
