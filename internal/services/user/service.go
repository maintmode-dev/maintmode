package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/useridentities"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// TokenRevoker revokes all active refresh tokens of a user. Blocking a user
// revokes their sessions, mirroring "/logout/all" for the target user.
type TokenRevoker interface {
	RevokeRefreshTokenByUserID(ctx context.Context, userID uuid.UUID) error
}

// UsersStore is the subset of the users storage the service depends on.
// Defined here (consumer side) so tests can substitute fakes — notably the
// CountActiveAdmins used by the last-admin lockout guard.
type UsersStore interface {
	Create(ctx context.Context, u *entity.User) (*entity.User, error)
	GetByID(ctx context.Context, userID uuid.UUID) (*entity.User, error)
	GetByEmail(ctx context.Context, email string) (*entity.User, error)
	GetForUpdateByID(ctx context.Context, userID uuid.UUID) (*entity.User, error)
	List(ctx context.Context, cmd *entity.ListUsersCmd) ([]*entity.User, int64, error)
	Update(ctx context.Context, user *entity.User) error
	CountActiveAdmins(ctx context.Context) (int64, error)
	LockAdminMutations(ctx context.Context) error
}

// AuditPublisher enqueues an audited action to the durable outbox. Defined
// consumer-side so the user service depends only on the publish capability and
// can be tested with a fake.
type AuditPublisher interface {
	Publish(ctx context.Context, action audit.Action) error
}

// LoginProviderResolver turns a login provider's system name into the id of the
// integration_settings row behind it.
//
// Declared consumer-side, and it has to be: the module boundaries forbid this
// module from importing the integration storages directly (see .golangci.yaml,
// module-auth-stores), so bootstrap injects the integration service behind this
// one method -- the mirror of how that module reaches this one's store.
//
// A name that resolves to nothing is an error, never a fallback to the built-in
// branch: quietly demoting an unresolvable provider would create an account
// outside any provider at all.
type LoginProviderResolver interface {
	ResolveID(ctx context.Context, name entity.AuthMethod) (uuid.UUID, error)
}

// SeatGuard is the seats-cap guard the role-granting paths call inside their
// mutation tx before persisting. Defined consumer-side (a subset of
// license.Enforcement) so the user service depends only on the guard and can be
// tested with a fake; self-hosted wires license.Noop, which is a no-op.
type SeatGuard interface {
	EnsureSeatAvailable(ctx context.Context) error
}

// Service manages user-related operations including role management.
type Service struct {
	txManager       *dbtx.TxManager
	usersStore      UsersStore
	identitiesStore *useridentities.Store
	auditPublisher  AuditPublisher
	tokenRevoker    TokenRevoker
	seatGuard       SeatGuard
	// providers resolves a login provider name to its registry row id. Wired by
	// WithLoginProviderResolver rather than taken as a constructor argument:
	// NewServices builds this service before the integration service exists.
	providers LoginProviderResolver
	// allowOpenSignup lets an unknown, uninvited user self-register as guest on
	// OAuth login (cfg.Auth.AllowOpenSignup, read once at wiring time).
	allowOpenSignup bool
}

func NewService(
	txManager *dbtx.TxManager,
	usersStore UsersStore,
	identitiesStore *useridentities.Store,
	auditPublisher AuditPublisher,
	tokenRevoker TokenRevoker,
	seatGuard SeatGuard,
	allowOpenSignup bool,
) *Service {
	return &Service{
		auditPublisher:  auditPublisher,
		txManager:       txManager,
		usersStore:      usersStore,
		identitiesStore: identitiesStore,
		tokenRevoker:    tokenRevoker,
		seatGuard:       seatGuard,
		allowOpenSignup: allowOpenSignup,
	}
}

// WithLoginProviderResolver wires the registry lookup the sign-in and link
// paths need. A setter rather than a constructor argument because NewServices
// builds this service before the integration service that implements it.
func (s *Service) WithLoginProviderResolver(providers LoginProviderResolver) *Service {
	s.providers = providers

	return s
}

// methodRef addresses a sign-in method for the store: a built-in method by
// name, anything else by the id of the registry row that vouches for it.
//
// The branch is decided from the method itself, never from a failed lookup. A
// registry name that resolves to nothing is an error the caller must see -- the
// alternative, treating it as built-in, would write an identity that no
// provider stands behind.
func (s *Service) methodRef(ctx context.Context, method entity.AuthMethod) (entity.SignInMethodRef, error) {
	if method.IsBuiltin() {
		return entity.SignInByBuiltin(method), nil
	}

	if s.providers == nil {
		// Misconfiguration, not a runtime condition: a binary that signs people
		// in through registry providers was wired without the resolver. Said
		// loudly here rather than degrading to the built-in branch.
		return entity.SignInMethodRef{}, fmt.Errorf("%w: login provider resolver is not wired", apperr.ErrUnsupportedProvider)
	}

	id, err := s.providers.ResolveID(ctx, method)
	if errors.Is(err, apperr.ErrIntegrationNotFound) {
		// The method is registered in the auth snapshot but its registry row is
		// gone. That is a normal window rather than a corrupt state: the
		// reloader rebuilds asynchronously, and it deliberately keeps the
		// previous snapshot live when a rebuild fails -- so a deleted provider
		// can stay listed for a while.
		//
		// Reported as "unsupported provider" (400), NOT as the underlying
		// ErrIntegrationNotFound. That one maps to 404, which would tell the
		// person signing in that the thing they asked for does not exist --
		// while what they named was a provider the instance was still
		// advertising. The 404 is addressed to an admin reading about an
		// integration; the caller here is a user holding a login button.
		return entity.SignInMethodRef{}, fmt.Errorf("%w: %s", apperr.ErrUnsupportedProvider, method)
	}
	if err != nil {
		return entity.SignInMethodRef{}, fmt.Errorf("resolve login provider %q: %w", method, err)
	}

	return entity.SignInByIntegration(id), nil
}

// publishAudit publishes an audited action to the durable outbox. A failed
// enqueue is logged, not propagated: the user-management action already
// committed, and must not be reported as failed because the audit publish
// hiccuped. The durability guarantee is "once enqueued, it survives a crash".
func (s *Service) publishAudit(ctx context.Context, action audit.Action) {
	if err := s.auditPublisher.Publish(ctx, action); err != nil {
		xlog.Error(ctx, "failed to publish audit action",
			xfield.String("action", fmt.Sprintf("%T", action)),
			xfield.Error(err),
		)
	}
}

func (s *Service) updateWithApply(ctx context.Context, userID uuid.UUID, fn func(ctx context.Context, user *entity.User) error) (*entity.User, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.applyUserForUpdate")
	defer span.End()

	var user *entity.User
	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		user, err = s.usersStore.GetForUpdateByID(ctx, userID)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}

		if err = fn(ctx, user); err != nil {
			return fmt.Errorf("apply user update: %w", err)
		}

		err = s.usersStore.Update(ctx, user)
		if err != nil {
			return fmt.Errorf("update user: %w", err)
		}

		return nil
	})
	if err != nil {
		xlog.Error(ctx, "failed to update user", xfield.Error(err))
		return nil, err
	}

	return user, nil
}

// ensureNotLastActiveAdmin returns ErrLastAdmin when removing/blocking this user
// would leave the organization without an active admin. It is a no-op for users
// that are not active admins. Call this inside the update transaction, after the
// target row's FOR UPDATE lock (every caller must keep that "row lock ->
// advisory lock" order so lock acquisition can never cycle).
func (s *Service) ensureNotLastActiveAdmin(ctx context.Context, user *entity.User) error {
	if !user.IsActiveAdmin() {
		return nil
	}

	// Serialize the count-then-write decision across concurrent admin
	// mutations — without the lock the guard is prone to write-skew (mechanics
	// on dbtx.AdvisoryLockKeyAdminMutations). The xact-scoped lock is released
	// on commit/rollback; a waiting transaction then recounts and observes the
	// committed state.
	if err := s.usersStore.LockAdminMutations(ctx); err != nil {
		return fmt.Errorf("lock admin mutations: %w", err)
	}

	admins, err := s.usersStore.CountActiveAdmins(ctx)
	if err != nil {
		return fmt.Errorf("count active admins: %w", err)
	}
	if admins <= 1 {
		return apperr.ErrLastAdmin
	}

	return nil
}
