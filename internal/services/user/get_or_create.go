package user

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
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
// branch and decides whether this login may create the user. The first-admin
// grant on a zero-admin instance is a plain AssignRoles(admin), whose
// RolesChanged(actor=system) records the promotion in the audit log — except on
// the break-glass path, which grants admin without AssignRoles (see
// createWithIdentity) and publishes that event itself.
func (s *Service) createByPolicy(ctx context.Context, provider entity.AuthMethod, info *entity.OAuthProviderUserInfo, policy entity.UserCreationPolicy) (*entity.User, error) {
	// Break-glass into an account that already exists under another provider.
	// users.email is NOT NULL UNIQUE, so creating a second row for the same
	// person would fail the index — and the unique-violation recovery upstream
	// only catches the IDENTITY index, not this one. That failure would land
	// exactly on the operator whose provider broke, which is who break-glass
	// exists for. Checked before the zero-admin branch: on a zero-admin instance
	// with a matching email both branches would end at admin anyway, and linking
	// keeps one human to one row.
	if provider == entity.AuthMethodBootstrap {
		user, err := s.linkBootstrapToExistingUser(ctx, info)
		if err != nil {
			return nil, err
		}
		if user != nil {
			return user, nil
		}
	}

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

	// The seats cap must not stand between an operator and a break-glass login:
	// a credential that stops working when the license runs out of seats fails
	// exactly when the existing admins are unreachable AND the org is at its
	// cap. The exemption is keyed on the PROVIDER rather than on a branch,
	// because the zero-admin branch is shared verbatim with Google — exempting
	// the branch would silently lift the cap for the Google first-admin path
	// too, which is a separate pre-existing defect and not this change's to fix.
	if provider == entity.AuthMethodBootstrap {
		user, err = s.grantRolesUnguarded(ctx, user.ID, grantRoles)
		if err != nil {
			return nil, fmt.Errorf("grant bootstrap roles: %w", err)
		}
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

// linkBootstrapToExistingUser attaches the break-glass identity to a user that
// already exists under the configured email, returning nil when there is no
// such user (the caller then falls through to ordinary creation).
//
// The identity row is written through the store directly rather than through
// Service.LinkIdentity: LinkIdentity pre-checks by subject and answers
// ErrProviderLinkedToAnotherUser — a 409 that the unique-violation recovery in
// GetOrCreateByAuthInfo does not catch. Going through the store means a
// concurrent racer hits the (provider, subject) index instead and lands in that
// existing recovery, which re-resolves by identity and returns the winner's
// user. That is what keeps two simultaneous break-glass logins deterministic.
func (s *Service) linkBootstrapToExistingUser(ctx context.Context, info *entity.OAuthProviderUserInfo) (*entity.User, error) {
	user, err := s.usersStore.GetByEmail(ctx, info.Email)
	if errors.Is(err, apperr.ErrUserNotFound) {
		return nil, nil //nolint:nilnil // "no such user" is a branch signal, not an error
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}

	_, err = s.identitiesStore.Create(ctx, &entity.UserIdentity{
		UserID:   user.ID,
		Provider: entity.AuthMethodBootstrap,
		Subject:  info.ID,
		Email:    info.Email,
	})
	if err != nil {
		return nil, fmt.Errorf("create bootstrap identity: %w", err)
	}

	user, err = s.grantRolesUnguarded(ctx, user.ID, []entity.Role{entity.RoleAdmin})
	if err != nil {
		return nil, fmt.Errorf("grant bootstrap admin: %w", err)
	}

	return user, nil
}

// grantRolesUnguarded unions roles onto the user without consulting the seats
// cap. It is the break-glass counterpart to AssignRoles and deliberately keeps
// everything else that method does — skipping the cap is the ONLY difference.
//
// It goes through updateWithApply rather than writing the caller's snapshot,
// and that is a correctness requirement, not symmetry. usersStore.Update is a
// blind full-column write: it persists Roles, BlockedAt, Timezone and the
// messenger tags from whatever struct it is handed. Writing a snapshot read
// earlier — the link branch reads its user with a plain GetByEmail — would
// resurrect every one of those fields as they looked at read time. Concretely:
// an admin blocking this user while a break-glass login is in flight would have
// blocked_at silently reset to NULL by the login's own write, un-blocking the
// account. updateWithApply takes the SELECT ... FOR UPDATE that BlockUser also
// takes, so the two serialize and the closure below sees committed state.
//
// The kept behaviors: roles the user already holds are a no-op, and the
// RolesChanged(actor=system) audit record is published only when the role set
// actually changes — a repeat break-glass login into an already-admin account
// must not fabricate a promotion that did not happen.
//
// It runs inside the caller's transaction: TxManager.WithinTx is reentrant, so
// the role update commits atomically with the identity row that preceded it.
func (s *Service) grantRolesUnguarded(ctx context.Context, userID uuid.UUID, roles []entity.Role) (*entity.User, error) {
	var added []entity.Role

	user, err := s.updateWithApply(ctx, userID, func(_ context.Context, user *entity.User) error {
		added = lo.Filter(roles, func(role entity.Role, _ int) bool {
			return !slices.Contains(user.Roles, role)
		})
		if len(added) == 0 {
			return apperr.ErrNotChanged
		}

		user.Roles = append(slices.Clone(user.Roles), added...)
		return nil
	})
	if errors.Is(err, apperr.ErrNotChanged) {
		// Already holds every role: no write, no audit record, and the current
		// user is what the caller needs.
		return s.GetByID(ctx, userID)
	}
	if err != nil {
		return nil, fmt.Errorf("update user roles: %w", err)
	}

	s.publishAudit(ctx, audit.RolesChanged{
		Actor:  entity.SystemUser,
		Target: user,
		Kind:   audit.RolesAssigned,
		Change: audit.RolesChange{Roles: added},
	})

	return user, nil
}
