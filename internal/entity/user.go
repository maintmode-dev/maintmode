package entity

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
)

type Role string

const (
	RoleGuest    Role = "guest"
	RoleEditor   Role = "editor"
	RoleReviewer Role = "reviewer"
	RoleAdmin    Role = "admin"
	// system admin role is used for system-level operations
	roleSystemAdmin Role = "system admin"
)

var rolesLevel = map[Role]int{
	RoleGuest:    0,
	RoleEditor:   1,
	RoleReviewer: 2,
	RoleAdmin:    3,
}

// RoleOccupiesSeat reports whether a role consumes a licensed seat: any role
// ranked above guest (admin/reviewer/editor). Guest, empty and unknown roles do
// not — an unknown role is absent from rolesLevel and reads as level 0, the same
// as guest. Derived from the rolesLevel ordering so there is one source of truth
// for "which roles are seat-occupying".
func RoleOccupiesSeat(role Role) bool {
	return rolesLevel[role] > rolesLevel[RoleGuest]
}

// Valid checks whether the role is one of the known roles.
func (r Role) Valid(ctx context.Context) bool {
	switch r {
	case RoleGuest, RoleEditor, RoleReviewer, RoleAdmin:
		return true
	}

	xlog.Error(ctx, "invalid role", xfield.Any("role", r))
	return false
}

var DefaultRoles = []Role{RoleGuest}

// ApproverEligibleRoles are the roles that make a user eligible to be assigned as
// a maintenance approver. It mirrors the RBAC right maintenance.approve: reviewer
// and admin (which inherits reviewer).
var ApproverEligibleRoles = []Role{RoleReviewer, RoleAdmin}

type User struct {
	ID        uuid.UUID
	Email     string
	Name      string
	Roles     []Role
	CreatedAt time.Time
	// BlockedAt is nil for active users and records when an admin blocked the
	// user. Blocking preserves roles so unblock immediately restores access.
	BlockedAt *time.Time
	// Timezone is the user's preferred IANA timezone (e.g. "Asia/Nicosia").
	// nil means "not selected" — the frontend falls back to browser auto-detect.
	// Stored as an IANA string (never a numeric offset) so it survives DST.
	Timezone *string
}

// IsBlocked reports whether the user is currently blocked.
func (u *User) IsBlocked() bool { return u.BlockedAt != nil }

// IsAdmin reports whether the user holds the admin role (regardless of blocked state).
func (u *User) IsAdmin() bool { return slices.Contains(u.Roles, RoleAdmin) }

// IsActiveAdmin reports whether the user is an admin and not blocked. Only active
// admins count toward last-admin lockout protection.
func (u *User) IsActiveAdmin() bool { return u.IsAdmin() && !u.IsBlocked() }

var SystemUser = &User{
	Email: "system@email.com",
	Name:  "system",
	Roles: []Role{roleSystemAdmin},
}

// UnknownUserName labels an author whose profile could not be resolved from the
// auth service (auth unavailable, or the user no longer exists). Read paths
// degrade to this rather than failing: the maintenance still renders its author
// slot, carrying the id with a human-readable placeholder name.
const UnknownUserName = "Unknown user"

type UserSummary struct {
	ID    uuid.UUID
	Name  string
	Email string
}

// ToUserSummary projects a resolved user onto the summary shape.
func (u *User) ToUserSummary() *UserSummary {
	return &UserSummary{ID: u.ID, Name: u.Name, Email: u.Email}
}

// NewUnknownUserSummary builds the degraded, id-only summary used when auth
// cannot resolve a user. The id is preserved so callers can still correlate.
func NewUnknownUserSummary(id uuid.UUID) *UserSummary {
	return &UserSummary{ID: id, Name: UnknownUserName}
}
