package user

import (
	"context"
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
