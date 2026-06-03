package user

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// ListResult is a page of users plus the metadata the API layer needs to render
// last-admin lockout state.
type ListResult struct {
	Users []*entity.User
	Total int64
	// ActiveAdminCount is the number of non-blocked admins in the whole system,
	// used to compute is_last_admin per row.
	ActiveAdminCount int64
}

// ListUsers returns a paginated, filtered list of users (admin-only use-case).
func (s *Service) ListUsers(ctx context.Context, cmd *entity.ListUsersCmd) (*ListResult, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.ListUsers")
	defer span.End()

	users, total, err := s.usersStore.List(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "failed to list users", xfield.Error(err))
		return nil, err
	}

	activeAdmins, err := s.usersStore.CountActiveAdmins(ctx)
	if err != nil {
		xlog.Error(ctx, "failed to count active admins", xfield.Error(err))
		return nil, err
	}

	return &ListResult{
		Users:            users,
		Total:            total,
		ActiveAdminCount: activeAdmins,
	}, nil
}
