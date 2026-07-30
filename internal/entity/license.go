package entity

import "time"

// LicenseStatus is the status Console reports in the heartbeat response. The
// instance treats it as authoritative: it blocks only on an explicit blocked
// status, never on transport failures.
type LicenseStatus string

const (
	LicenseStatusActive  LicenseStatus = "active"
	LicenseStatusBlocked LicenseStatus = "blocked"
)

// License is the last heartbeat response cached in license_cache. When Console
// is unreachable the instance keeps operating on this snapshot indefinitely —
// an admin-panel outage must never degrade a customer.
//
// The row is seeded (by the migration) with Status active before Console has
// ever answered, so the block gate has a status to read from the first request.
// SeatsPurchased and FetchedAt are Console-owned: they are nil until the first
// heartbeat fills them, which is why both are pointers — a nil is honestly
// "Console has not reported this yet", not a zero seat count or the epoch.
type License struct {
	Status         LicenseStatus
	SeatsPurchased *int64
	// FetchedAt records when this license was last confirmed by Console; nil
	// before the first heartbeat.
	FetchedAt *time.Time
}

// Blocked reports whether the license shuts the instance down: an explicit
// blocked status from Console. Anything else (active, unknown, nil) is treated
// as not blocked — enforcement never fails closed on a transport or decode
// hiccup.
func (l *License) Blocked() bool {
	return l != nil && l.Status == LicenseStatusBlocked
}

// SeatBucket counts seats of one role: active (non-blocked) users and pending
// (not yet accepted) invitations.
type SeatBucket struct {
	Active  int64
	Pending int64
}

// SeatUsage is the per-role seat report sent in the heartbeat. A user (or
// invitation) holding several roles counts exactly once, in the bucket of its
// highest role (admin > reviewer > editor > guest), so the buckets are disjoint
// and TotalActive is an honest cap invariant: sum(Active) == TotalActive.
type SeatUsage struct {
	Admin    SeatBucket
	Reviewer SeatBucket
	Editor   SeatBucket
	Guest    SeatBucket
}

// TotalActive is the number of active users across all roles, guests included.
// Reporting-only, sent to Console in the heartbeat.
func (u SeatUsage) TotalActive() int64 {
	return u.SeatsUsed() + u.Guest.Active
}

// SeatsUsed is the number of active users that occupy a licensed seat, reported
// to Console in the heartbeat. A seat is occupied by roles that can modify data
// (admin/reviewer/editor); guests are read-only and do not occupy a seat.
func (u SeatUsage) SeatsUsed() int64 {
	return u.Admin.Active + u.Reviewer.Active + u.Editor.Active
}

// SeatsOccupied is the number of seats consumed for the cap guard: active
// seat-users PLUS live pending seat-invites. It is deliberately distinct from
// SeatsUsed (Active-only, heartbeat report): a pending invite reserves a seat
// from send time (Variant A), so the cap check must count both. Guests never
// occupy a seat.
func (u SeatUsage) SeatsOccupied() int64 {
	return u.Admin.Active + u.Admin.Pending +
		u.Reviewer.Active + u.Reviewer.Pending +
		u.Editor.Active + u.Editor.Pending
}

// SeatsPending is the number of seats held by live pending invites — the
// pending half of SeatsOccupied. Guests never hold a seat.
//
// Deliberately summed rather than derived as SeatsOccupied() - SeatsUsed():
// that difference happens to be equal today, but it couples this counter to two
// definitions that are intentionally different (SeatsUsed is the Console-owned
// report, SeatsOccupied is the cap guard). If the reported definition moves, the
// difference would silently stop meaning "live pending invites".
func (u SeatUsage) SeatsPending() int64 {
	return u.Admin.Pending + u.Reviewer.Pending + u.Editor.Pending
}

// HeartbeatReport is the instance-side half of the heartbeat contract: seat
// usage, last domain activity and the running build version. Collected fresh
// at send time. LastActivityAt is nil for an instance with no domain activity
// yet (Console renders it as "—").
type HeartbeatReport struct {
	SeatsUsed      SeatUsage
	LastActivityAt *time.Time
	Version        string
}

// HighestRole picks the seat bucket for a role set: admin > reviewer > editor,
// everything else (guest, empty, unknown) falls into guest. Every holder lands
// in exactly one bucket, which keeps the buckets disjoint and the cap invariant
// honest.
func HighestRole(roles []Role) Role {
	highestRole, roleLevel := RoleGuest, rolesLevel[RoleGuest]

	for _, role := range roles {
		if level := rolesLevel[role]; level > roleLevel {
			highestRole, roleLevel = role, level
		}
	}

	return highestRole
}

// BucketSeats builds the heartbeat seat report from the role sets of active
// users and live pending invitations. Each row counts once, in the bucket of
// its highest role.
func BucketSeats(activeRoles, pendingRoles [][]Role) SeatUsage {
	var usage SeatUsage

	bucket := func(roles []Role) *SeatBucket {
		switch HighestRole(roles) {
		case RoleAdmin:
			return &usage.Admin
		case RoleReviewer:
			return &usage.Reviewer
		case RoleEditor:
			return &usage.Editor
		default:
			return &usage.Guest
		}
	}

	for _, roles := range activeRoles {
		bucket(roles).Active++
	}
	for _, roles := range pendingRoles {
		bucket(roles).Pending++
	}
	return usage
}
