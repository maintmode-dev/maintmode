// Package userpicker serves lists of users selectable in maintenance assignment
// pickers (reviewer/owner, notify targets). The users table is owned by the
// auth service, so this service does not query a local store — it reads active
// (non-blocked) users from auth over S2S via the auth gateway. The maintmode
// domain notion of "assignable" lives here; auth only exposes a neutral active-
// users listing.
package userpicker

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	authgateway "github.com/ruko1202/maintmode/internal/gateways/auth"
)

// AuthGateway is the subset of the auth gateway the picker depends on.
type AuthGateway interface {
	ListActiveUsers(ctx context.Context, q *entity.ListAssignableUsersQuery) (*authgateway.ListActiveUsersResult, error)
}

// Service lists users assignable to a maintenance.
type Service struct {
	authGateway AuthGateway
}

func NewService(authGateway AuthGateway) *Service {
	return &Service{authGateway: authGateway}
}

// ListResult is a page of assignable users.
type ListResult struct {
	Users  []*entity.User
	Total  int64
	Limit  int64
	Offset int64
}

// ListAssignable returns active users eligible for maintenance assignment.
func (s *Service) ListAssignable(ctx context.Context, q *entity.ListAssignableUsersQuery) (*ListResult, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.UserPicker.ListAssignable")
	defer span.End()

	res, err := s.authGateway.ListActiveUsers(ctx, q)
	if err != nil {
		xlog.Error(ctx, "failed to list assignable users", xfield.Error(err))
		return nil, err
	}

	return &ListResult{Users: res.Users, Total: res.Total, Limit: res.Limit, Offset: res.Offset}, nil
}
