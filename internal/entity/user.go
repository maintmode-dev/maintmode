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
)

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

type User struct {
	ID        uuid.UUID
	Email     string
	Name      string
	Roles     []Role
	CreatedAt time.Time
	// BlockedAt is nil for active users and records when an admin blocked the
	// user. Blocking preserves roles so unblock immediately restores access.
	BlockedAt *time.Time
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
	Roles: []Role{RoleAdmin},
}
