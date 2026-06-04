package user

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

// ListUsers returns a paginated, filtered list of users (admin-only use-case).
func (s *Service) ListUsers(ctx context.Context, cmd *entity.ListUsersCmd) (*entity.ListUsersResult, error) {
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

	userIDs := lo.Map(users, func(u *entity.User, _ int) uuid.UUID { return u.ID })
	providersByUser, err := s.identitiesStore.ListProvidersByUserIDs(ctx, userIDs)
	if err != nil {
		xlog.Error(ctx, "failed to list connected providers for users", xfield.Error(err))
		return nil, err
	}

	return &entity.ListUsersResult{
		Users:            users,
		Total:            total,
		ActiveAdminCount: activeAdmins,
		ProvidersByUser:  providersByUser,
	}, nil
}
