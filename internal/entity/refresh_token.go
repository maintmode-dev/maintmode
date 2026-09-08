package entity

import (
	"time"

	"github.com/google/uuid"
)

const DefaultGraceTTL = 30 * time.Second

// RefreshToken represents a refresh token in the rotation system.
//
// Lifecycle:
//
//	Login  → Token_1 (Family=X, Revoked=false)
//	Refresh → Token_1 (Revoked=true, GraceTTL=+30s, ReplacedBy=Token_2)
//	        → Token_2 (Family=X, Revoked=false)
//	After 30 days → ExpiresAt → all family tokens expired → re-login required
type RefreshToken struct {
	// Token is SHA256 hash of random 32 bytes. Client receives the "raw" token,
	// only the hash is stored in the database. Tokens are useless in case of DB leak.
	Token string

	// UserID is the owner's UUID. Used for RevokeByUserID (logout all).
	UserID uuid.UUID

	// Family is the login session UUID. All tokens in one rotation chain share
	// the same Family. On reuse detection, the entire family is revoked.
	Family uuid.UUID

	// ExpiresAt is retained for the row's own bookkeeping but is NOT what
	// decides whether a session is alive: two policy limits do, and they are
	// evaluated against timestamps rather than against a stored deadline.
	// Keeping the deadline in the row would freeze the policy at issue time, so
	// changing it would only affect sessions created afterwards.
	ExpiresAt time.Time

	// SessionStartedAt is when the SESSION began -- carried unchanged across
	// every rotation in the chain. CreatedAt cannot serve this purpose: rotation
	// inserts a new row, so CreatedAt marks the last rotation.
	//
	// The two together answer two independent questions: has the session existed
	// too long (SessionStartedAt vs the maximum lifetime), and has it been idle
	// too long (CreatedAt vs the inactive lifetime).
	SessionStartedAt time.Time

	// GraceTTL is the grace window (30 sec) after rotation to handle the
	// multi-tab problem. While time.Now() < GraceTTL, a revoked token
	// is still accepted and returns an access token via ReplacedBy.
	// After expiration, reuse detection triggers revocation of the entire family.
	GraceTTL *time.Time

	// Revoked is true after rotation or logout. By itself it doesn't block —
	// must be checked together with GraceTTL.
	Revoked bool

	// ReplacedBy is the hash of the successor token. Used in grace period:
	// server finds the replacement and issues a new access token
	// (but without a new refresh token — client uses the one already received).
	ReplacedBy *string

	// BoundIP is the client's IP at issuance. If refresh comes from a different IP,
	// the entire family is revoked, returning ErrSuspiciousActivity.
	BoundIP string

	// CreatedAt is the creation time of this specific token in the rotation chain.
	CreatedAt time.Time
	// UpdatedAt is the update time of this specific token in the rotation chain.
	UpdatedAt time.Time
}
