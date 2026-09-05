// Package otp issues one-time sign-in codes, hands them to the queue for
// delivery by email, and redeems them.
//
// It owns the credential: issuance, the per-code attempt ceiling, expiry and the
// session-nonce comparison. It deliberately does NOT issue tokens -- Verify
// reports the user it resolved and the auth service runs it through the same
// IssueTokenPair funnel as every other sign-in method, so blocking, audit and IP
// binding apply here without being restated.
package otp

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Store is the credential persistence this service needs. Defined consumer-side
// so the service depends on the three operations it performs, not on the whole
// store, and so tests can substitute a fake.
type Store interface {
	Create(ctx context.Context, cred *entity.AuthCredential) (*entity.AuthCredential, error)
	// GetUnconsumedOTPByUserIDForUpdate, not the unlocked twin: this read decides
	// whether to retire what it finds, so it must not race a concurrent claim.
	GetUnconsumedOTPByUserIDForUpdate(ctx context.Context, userID uuid.UUID) (*entity.AuthCredential, error)
	ConsumeOTP(ctx context.Context, id uuid.UUID) (bool, error)
	// The unlocked read is the verify path's: it takes no transaction, so there
	// is no lock to hold and nothing for it to protect against.
	GetUnconsumedOTPByUserID(ctx context.Context, userID uuid.UUID) (*entity.AuthCredential, error)
	ClaimOTPAttempt(ctx context.Context, id uuid.UUID, maxAttempts int16) (bool, error)
}

// UserService resolves the address to a user. Only the lookup is needed.
type UserService interface {
	GetByEmail(ctx context.Context, email string) (*entity.User, error)
}

// Keyring seals the per-task data key under the active KEK.
//
// Consumer-side because the auth module is barred from importing the data-key
// store, and because this service needs only the wrap half -- unwrapping happens
// in the delivery processor.
type Keyring interface {
	WrapDEK(dek []byte) (wrapped []byte, kekID string, err error)
}

// Cipher seals the code under that data key.
type Cipher interface {
	Encrypt(dek, plaintext, aad []byte) ([]byte, error)
}

// TaskScheduler enqueues the delivery task, joining the caller's transaction
// when one is on the context.
//
// The messaging sender facade cannot be used here: it accepts a NotifyMessage
// and builds the shared notify payload itself, so there is no way to hand it the
// sealed-code payload this flow needs. Going to the scheduler directly is what
// keeps the code out of the queue in plaintext.
type TaskScheduler interface {
	Schedule(ctx context.Context, taskType string, payload any, idempotencyKey string) (uuid.UUID, error)
}

// Service issues one-time sign-in codes.
type Service struct {
	txManager *dbtx.TxManager
	store     Store
	userSrv   UserService
	keyring   Keyring
	cipher    Cipher
	scheduler TaskScheduler
	ttl       time.Duration
	// maxAttempts is shared with the verify path, which reads it through
	// MaxAttempts() rather than resolving its own copy from config. The two
	// enforce complementary halves of one rule and must not disagree.
	maxAttempts int16
}

func NewService(
	cfg *config.AppConfig,
	txManager *dbtx.TxManager,
	store Store,
	userSrv UserService,
	keyring Keyring,
	cipher Cipher,
	sched TaskScheduler,
) *Service {
	return &Service{
		txManager:   txManager,
		store:       store,
		userSrv:     userSrv,
		keyring:     keyring,
		cipher:      cipher,
		scheduler:   sched,
		ttl:         TTL(cfg.Auth),
		maxAttempts: MaxAttempts(cfg.Auth),
	}
}

// TTL exposes the configured code lifetime.
func (s *Service) TTL() time.Duration { return s.ttl }

// MaxAttempts exposes the configured guess ceiling, mirroring TTL. It is an
// accessor rather than a package-level call at each site so that the verify
// service can depend on the resolved number without re-resolving config — the
// divergence MaxAttempts documents.
func (s *Service) MaxAttempts() int16 { return s.maxAttempts }
