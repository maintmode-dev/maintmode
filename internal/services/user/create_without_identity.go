package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// CreateWithoutIdentity creates a user that signs in with a credential this
// backend holds itself -- a password -- and so has no external identity row.
// The user gets the default roles; anything more is granted by the caller.
//
// An address that already belongs to a user is refused with
// apperr.ErrUserAlreadyExists rather than resolved to that user: the caller is
// about to attach a credential, and attaching one to an existing account on the
// strength of an invitation link would let whoever holds the link take the
// account over. Checked case-insensitively first, since users.email is unique
// only as written; the unique index still answers a concurrent insert.
//
// It opens no transaction of its own, so the caller's transaction covers it.
func (s *Service) CreateWithoutIdentity(ctx context.Context, email, name string) (*entity.User, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.CreateWithoutIdentity")
	defer span.End()

	_, err := s.usersStore.GetByEmail(ctx, email)
	switch {
	case err == nil:
		return nil, apperr.ErrUserAlreadyExists
	case !errors.Is(err, apperr.ErrUserNotFound):
		return nil, fmt.Errorf("get user by email: %w", err)
	}

	user, err := s.usersStore.Create(ctx, &entity.User{
		Email: email,
		Name:  name,
		Roles: entity.DefaultRoles,
	})
	if err != nil {
		if dbtx.ErrorIs(err, dbtx.ErrPGUniqueViolation) {
			return nil, apperr.ErrUserAlreadyExists
		}
		xlog.Error(ctx, "create user without identity failed", xfield.Error(err))

		return nil, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}
