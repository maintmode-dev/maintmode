package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// GetOrCreateByOAuthInfo looks up a user by Google ID. If the user does not
// exist, it creates a new one with default "user" role.
func (s *Service) GetOrCreateByOAuthInfo(ctx context.Context, info *entity.OAuthProviderUserInfo) (*entity.User, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.GetOrCreateByOAuthInfo",
		xfield.String("email", info.Email),
	)
	defer span.End()

	var user *entity.User
	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		var err error

		user, err = s.usersStore.GetByOAuthProviderID(ctx, info.ID)
		switch {
		case errors.Is(err, apperr.ErrUserNotFound):
			user, err = s.usersStore.Create(ctx, &entity.User{
				Email:           info.Email,
				Name:            info.Name,
				OAuthProviderID: info.ID,
				Roles:           entity.DefaultRoles,
			})
			if err != nil {
				return fmt.Errorf("create user: %w", err)
			}
		case err != nil:
			return fmt.Errorf("get user: %w", err)
		}

		return nil
	})
	if err != nil {
		xlog.Error(ctx, "failed to get or create user", xfield.Error(err))
		return nil, err
	}

	user, err = s.assignAdminRoleBySystem(ctx, user)
	if err != nil {
		xlog.Error(ctx, "assign admin role failed", xfield.Error(err))
		return nil, err
	}

	return user, nil
}

// fixme: move to assign roles from `X-Test-Roles` request header
func (s *Service) assignAdminRoleBySystem(ctx context.Context, user *entity.User) (*entity.User, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.assignAdminRoleBySystem",
		xfield.String("email", user.Email),
	)
	defer span.End()

	if !s.env.IsDev() {
		return user, nil
	}

	err := s.AssignRole(ctx, &entity.AssignRoleCmd{
		Actor:  entity.SystemUser,
		UserID: user.ID,
		Role:   entity.RoleAdmin,
	})
	if err != nil {
		return nil, fmt.Errorf("assign admin role by system: %w", err)
	}

	user, err = s.GetByID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("get user after admin role assignment: %w", err)
	}

	return user, nil
}
