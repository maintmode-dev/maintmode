// Package otp issues one-time sign-in codes and hands them to the queue for
// delivery by email.
//
// It owns issuance only. Verifying a code, counting attempts against a ceiling
// and comparing the session nonce belong to the verify path, which is a separate
// piece of work; nothing here can be redeemed yet.
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
	GetUnconsumedOTPByUserID(ctx context.Context, userID uuid.UUID) (*entity.AuthCredential, error)
	ConsumeOTP(ctx context.Context, id uuid.UUID) (bool, error)
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
		txManager: txManager,
		store:     store,
		userSrv:   userSrv,
		keyring:   keyring,
		cipher:    cipher,
		scheduler: sched,
		ttl:       TTL(cfg.Auth),
	}
}

// TTL exposes the configured code lifetime.
func (s *Service) TTL() time.Duration { return s.ttl }
