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

	return user, nil
}
