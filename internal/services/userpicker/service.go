// Package userpicker serves lists of users selectable in maintenance assignment
// pickers (reviewer/owner, notify targets). The users table is owned by the
// auth service, so this service does not query a local store — it reads active
// (non-blocked) users from auth over S2S via the auth gateway. The maintmode
// domain notion of "assignable" lives here; auth only exposes a neutral active-
// users listing.
package userpicker

import (
	"context"

	"github.com/ruko1202/maintmode/internal/entity"
)

// ActiveUsersLister reads active (non-blocked) users from auth — the subset of
// the auth gateway the picker depends on.
type ActiveUsersLister interface {
	ListActiveUsers(ctx context.Context, q *entity.ListAssignableUsersQuery) (*entity.ListAssignableUsersResult, error)
}

// Service lists users assignable to a maintenance.
type Service struct {
	users ActiveUsersLister
}

func NewService(users ActiveUsersLister) *Service {
	return &Service{users: users}
}
