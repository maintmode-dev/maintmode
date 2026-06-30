package userpicker

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// ListAssignable returns active users eligible for maintenance assignment.
// "Assignable" is a maintmode domain notion: active (non-blocked) users,
// optionally filtered by search and roles. It maps that onto a neutral
// ListUsersCmd against the auth user service — ExcludeBlocked is always set so
// blocked users are never offered for assignment.
func (s *Service) ListAssignable(ctx context.Context, q *entity.ListAssignableUsersQuery) (*entity.ListAssignableUsersResult, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.UserPicker.ListAssignable")
	defer span.End()

	res, err := s.users.ListUsers(ctx, &entity.ListUsersCmd{
		Search:         q.Search,
		Roles:          q.Roles,
		Limit:          q.Limit,
		Offset:         q.Offset,
		ExcludeBlocked: true,
	})
	if err != nil {
		xlog.Error(ctx, "failed to list assignable users", xfield.Error(err))
		return nil, err
	}

	return &entity.ListAssignableUsersResult{
		Users:  res.Users,
		Total:  res.Total,
		Limit:  q.Limit,
		Offset: q.Offset,
	}, nil
}
