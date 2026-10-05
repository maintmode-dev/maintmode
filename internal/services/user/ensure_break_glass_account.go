package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// EnsureBreakGlassAccount returns the break-glass account, creating it on the
// first sign-in, and grants it admin on every sign-in past the seats cap, so
// neither a demotion nor a full license turns break-glass into a guest login.
// Of two racing first sign-ins, the loser hits the email index and retries.
func (s *Service) EnsureBreakGlassAccount(ctx context.Context, email, name string) (*entity.User, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.EnsureBreakGlassAccount")
	defer span.End()

	user, err := s.ensureBreakGlassAccount(ctx, email, name)
	if dbtx.ErrorIs(err, dbtx.ErrPGUniqueViolation) {
		user, err = s.ensureBreakGlassAccount(ctx, email, name)
	}

	return user, err
}

func (s *Service) ensureBreakGlassAccount(ctx context.Context, email, name string) (user *entity.User, err error) {
	err = s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		account, err := s.usersStore.GetByEmail(ctx, email)
		if errors.Is(err, apperr.ErrUserNotFound) {
			account, err = s.usersStore.Create(ctx, &entity.User{Email: email, Name: name, Roles: entity.DefaultRoles})
		}
		if err != nil {
			return fmt.Errorf("get or create break-glass account: %w", err)
		}

		user, err = s.grantRolesUnguarded(ctx, account.ID, []entity.Role{entity.RoleAdmin})

		return err
	})

	return user, err
}
