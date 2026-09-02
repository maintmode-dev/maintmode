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

// GetOrCreateByAuthInfo looks up a user by the provider identity (provider +
// subject). If no identity exists, the creation decision is: first-admin
// bootstrap (zero active admins) > policy.AllowCreate > open signup > refuse
// with apperr.ErrSignupDisabled, leaving zero rows behind. The whole decision
// runs in one transaction. Operational model: the operator logs in first,
// before anyone else can reach the instance, so concurrent first logins are
// excluded by assumption and the bootstrap decision needs no extra
// serialization.
func (s *Service) GetOrCreateByAuthInfo(ctx context.Context, provider entity.AuthMethod, info *entity.OAuthProviderUserInfo, policy entity.UserCreationPolicy) (*entity.User, error) {
	// Frozen telemetry identifier: the span name keeps its old spelling after the
	// method was renamed, so existing dashboards and saved queries keep matching.
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.GetOrCreateByOAuthInfo",
		xfield.String("provider", string(provider)),
		xfield.String("email", info.Email),
	)
	defer span.End()

	for _, role := range policy.GrantRoles {
		if !role.Valid(ctx) {
			return nil, apperr.ErrInvalidRole
		}
	}

	var user *entity.User
	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		user, err = s.getUserByIdentity(ctx, provider, info.ID)

		switch {
		case err == nil:
			// Existing user: an ordinary login is a pure lookup. Logging in never
			// escalates privileges — policy.GrantRoles applies only when the user
			// is first created (see createByPolicy). Any later role change is an
			// explicit admin action through AssignRoles/ReplaceRoles.
			return nil
		case errors.Is(err, apperr.ErrProviderNotConnected):
			// First time we see this identity: decide whether to create the user.
			user, err = s.createByPolicy(ctx, provider, info, policy)
			return err
		default:
			return fmt.Errorf("get identity: %w", err)
		}
	})
	if errors.Is(err, apperr.ErrProviderAlreadyConnected) {
		// A concurrent same-subject login won the race; the unique index rejected
		// our insert. The winner ran with the same policy and already created the
		// user with its roles, so we just look it up on a clean connection outside
		// the aborted transaction.
		return s.getUserByIdentity(ctx, provider, info.ID)
	}
	if err != nil {
		xlog.Error(ctx, "failed to get or create user", xfield.Error(err))
		return nil, err
	}

	return user, nil
}

// getUserByIdentity resolves the user owning a given provider identity, reading
// it as-is (no role changes). Used both on the ordinary login lookup and to
// recover the winner after a concurrent same-subject race.
func (s *Service) getUserByIdentity(ctx context.Context, provider entity.AuthMethod, subject string) (*entity.User, error) {
	identity, err := s.identitiesStore.GetByProviderSubject(ctx, provider, subject)
	if err != nil {
		return nil, fmt.Errorf("get identity after race: %w", err)
	}

	user, err := s.usersStore.GetByID(ctx, identity.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user after race: %w", err)
	}

	return user, nil
}

// createByPolicy runs inside the transaction on the "identity not found"
// branch and decides whether this login may create the user. The bootstrap
// grant is a plain AssignRoles(admin) — its RolesChanged(actor=system) already
// records the promotion in the audit log.
func (s *Service) createByPolicy(ctx context.Context, provider entity.AuthMethod, info *entity.OAuthProviderUserInfo, policy entity.UserCreationPolicy) (*entity.User, error) {
	admins, err := s.usersStore.CountActiveAdmins(ctx)
	if err != nil {
		return nil, fmt.Errorf("count active admins: %w", err)
	}

	switch {
	case admins == 0:
		// Bootstrap: the operator's first login on a zero-admin installation.
		return s.createWithIdentity(ctx, provider, info, []entity.Role{entity.RoleAdmin})
	case policy.AllowCreate:
		return s.createWithIdentity(ctx, provider, info, policy.GrantRoles)
	case s.allowOpenSignup:
		return s.createWithIdentity(ctx, provider, info, nil)
	default:
		// No invitation, signup closed: refuse loudly. The tx rolls back and
		// leaves zero rows behind. A same-subject login that races a creating
		// path is resolved by the unique-violation recovery on the create side,
		// so no re-check is needed here.
		return nil, apperr.ErrSignupDisabled
	}
}

// createWithIdentity inserts the user with the default roles plus its first
// provider identity, then unions grantRoles on top through AssignRoles — the
// same path (and RolesChanged audit, actor=system) as invitation roles.
func (s *Service) createWithIdentity(ctx context.Context, provider entity.AuthMethod, info *entity.OAuthProviderUserInfo, grantRoles []entity.Role) (*entity.User, error) {
	user, err := s.usersStore.Create(ctx, &entity.User{
		Email: info.Email,
		Name:  info.Name,
		Roles: entity.DefaultRoles,
	})
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	_, err = s.identitiesStore.Create(ctx, &entity.UserIdentity{
		UserID:   user.ID,
		Provider: provider,
		Subject:  info.ID,
		Email:    info.Email,
	})
	if err != nil {
		return nil, fmt.Errorf("create identity: %w", err)
	}

	if len(grantRoles) == 0 {
		return user, nil
	}

	user, err = s.AssignRoles(ctx, &entity.AssignRolesCmd{
		Actor:  entity.SystemUser,
		UserID: user.ID,
		Roles:  grantRoles,
	})
	if err != nil {
		return nil, fmt.Errorf("assign policy roles: %w", err)
	}

	return user, nil
}
