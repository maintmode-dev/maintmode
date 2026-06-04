package userpicker

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// ListAssignable returns active users eligible for maintenance assignment.
func (s *Service) ListAssignable(ctx context.Context, q *entity.ListAssignableUsersQuery) (*entity.ListAssignableUsersResult, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.UserPicker.ListAssignable")
	defer span.End()

	res, err := s.users.ListActiveUsers(ctx, q)
	if err != nil {
		xlog.Error(ctx, "failed to list assignable users", xfield.Error(err))
		return nil, err
	}

	return res, nil
}
