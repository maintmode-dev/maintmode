package entity

import (
	"context"
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
}

var SystemUser = &User{
	Email: "system@email.com",
	Name:  "system",
	Roles: []Role{RoleAdmin},
}
