// Package userpicker serves lists of users selectable in maintenance assignment
// pickers (reviewer/owner, notify targets). The users table is owned by the
// auth module, so this service does not query a local store — it reads active
// (non-blocked) users by calling the auth user service in-process. The maintmode
// domain notion of "assignable" lives here; the user service only exposes a
// neutral user listing, and this service translates "assignable" into the
// concrete ListUsersCmd filters.
package userpicker

import (
	"context"

	"github.com/ruko1202/maintmode/internal/entity"
)

// UserLister is the subset of the auth user service the picker depends on.
// Declared consumer-side so the picker can be tested with a mock; satisfied by
// *user.Service. The "assignable" composition (ExcludeBlocked, query mapping)
// lives in this package, not on the user service — see ListAssignable.
type UserLister interface {
	ListUsers(ctx context.Context, cmd *entity.ListUsersCmd) (*entity.ListUsersResult, error)
}

// Service lists users assignable to a maintenance.
type Service struct {
	users UserLister
}

func NewService(users UserLister) *Service {
	return &Service{users: users}
}
