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

// GetOrCreateByOAuthInfo looks up a user by the provider identity (provider +
// subject). If no identity exists, it creates a new user together with its
// first identity, atomically, with the default role.
func (s *Service) GetOrCreateByOAuthInfo(ctx context.Context, provider entity.OAuthProvider, info *entity.OAuthProviderUserInfo) (*entity.User, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.GetOrCreateByOAuthInfo",
		xfield.String("provider", string(provider)),
		xfield.String("email", info.Email),
	)
	defer span.End()

	var user *entity.User
	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		identity, err := s.identitiesStore.GetByProviderSubject(ctx, provider, info.ID)
		switch {
		case err == nil:
			user, err = s.usersStore.GetByID(ctx, identity.UserID)
			if err != nil {
				return fmt.Errorf("get user by identity: %w", err)
			}
			return nil
		case errors.Is(err, apperr.ErrProviderNotConnected):
			// No identity yet — create the user and its first identity.
			user, err = s.usersStore.Create(ctx, &entity.User{
				Email: info.Email,
				Name:  info.Name,
				Roles: entity.DefaultRoles,
			})
			if err != nil {
				return fmt.Errorf("create user: %w", err)
			}

			_, err = s.identitiesStore.Create(ctx, &entity.UserIdentity{
				UserID:   user.ID,
				Provider: provider,
				Subject:  info.ID,
				Email:    info.Email,
			})
			if err != nil {
				return fmt.Errorf("create identity: %w", err)
			}

			return nil
		default:
			return fmt.Errorf("get identity: %w", err)
		}
	})
	if errors.Is(err, apperr.ErrProviderAlreadyConnected) {
		// A concurrent first-login for the same subject won the race; the unique
		// index rejected our insert. The identity now exists, so resolve to the
		// winner's user. The lookup runs outside the aborted transaction.
		return s.getUserByIdentity(ctx, provider, info.ID)
	}
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

// getUserByIdentity resolves the user owning a given provider identity, applying
// the dev-mode admin role assignment like the create path. Used to recover from
// a concurrent first-login race.
func (s *Service) getUserByIdentity(ctx context.Context, provider entity.OAuthProvider, subject string) (*entity.User, error) {
	identity, err := s.identitiesStore.GetByProviderSubject(ctx, provider, subject)
	if err != nil {
		return nil, fmt.Errorf("get identity after race: %w", err)
	}

	user, err := s.usersStore.GetByID(ctx, identity.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user after race: %w", err)
	}

	return s.assignAdminRoleBySystem(ctx, user)
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

	user, err := s.AssignRoles(ctx, &entity.AssignRolesCmd{
		Actor:  entity.SystemUser,
		UserID: user.ID,
		Roles:  []entity.Role{entity.RoleAdmin},
	})
	if err != nil {
		return nil, fmt.Errorf("assign admin role by system: %w", err)
	}

	return user, nil
}
