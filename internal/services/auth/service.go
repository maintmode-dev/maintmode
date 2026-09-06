package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/authmethod"
	"github.com/ruko1202/maintmode/internal/services/token"
	"github.com/ruko1202/maintmode/internal/services/user"
	"github.com/ruko1202/maintmode/internal/storages/blacklisttoken"
	"github.com/ruko1202/maintmode/internal/storages/distributedlock"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// AuditPublisher enqueues an audited action to the durable outbox. Defined
// consumer-side so the auth service depends only on the publish capability and
// can be tested with a fake.
type AuditPublisher interface {
	Publish(ctx context.Context, action audit.Action) error
}

// OTPRequester issues a one-time code. The reset flow reuses the sign-in code
// mechanism unchanged rather than growing a second one.
type OTPRequester interface {
	Request(ctx context.Context, email string) (string, error)
}

// OTPVerifier redeems a one-time code and reports the user it belonged to.
//
// Consumer-side, and narrow on purpose: this service needs the redemption and
// nothing else about one-time codes. The audit reason comes back beside the
// error because the endpoint answers every failure identically, so the reason
// has nowhere else to go.
type OTPVerifier interface {
	Verify(ctx context.Context, cmd *entity.VerifyOTPCmd) (*entity.User, entity.AuditFailureReason, error)
}

// PasswordCredentials reads and writes the stored password hash. Consumer-side
// and narrow: this service verifies one and replaces one, and needs nothing
// else about the credentials table.
type PasswordCredentials interface {
	GetPasswordByUserID(ctx context.Context, userID uuid.UUID) (*entity.AuthCredential, error)
	UpsertPassword(ctx context.Context, userID uuid.UUID, phc string) error
}

type Service struct {
	cfg            *config.JWT
	txManager      *dbtx.TxManager
	usersSrv       *user.Service
	tokenSrv       *token.Service
	authMethods    *authmethod.Methods
	locker         *distributedlock.Store
	blacklistStore *blacklisttoken.Store
	auditPublisher AuditPublisher
	otpVerifier    OTPVerifier
	otpRequester   OTPRequester
	passwords      PasswordCredentials
}

func NewService(
	cfg *config.JWT,
	txManager *dbtx.TxManager,
	usersSrv *user.Service,
	locker *distributedlock.Store,
	blacklistStore *blacklisttoken.Store,
	authMethods *authmethod.Methods,
	tokenSvc *token.Service,
	auditPublisher AuditPublisher,
	otpVerifier OTPVerifier,
	otpRequester OTPRequester,
	passwords PasswordCredentials,
) *Service {
	return &Service{
		cfg:            cfg,
		txManager:      txManager,
		usersSrv:       usersSrv,
		locker:         locker,
		blacklistStore: blacklistStore,
		authMethods:    authMethods,
		tokenSrv:       tokenSvc,
		auditPublisher: auditPublisher,
		otpVerifier:    otpVerifier,
		otpRequester:   otpRequester,
		passwords:      passwords,
	}
}

// publishAudit publishes an audited action to the durable outbox. A failed
// enqueue is logged, not propagated: the user's auth action must not fail
// because the audit publish hiccuped. The durability guarantee is "once
// enqueued, it survives a crash" — a publish failure is a loud log, not a lost
// request.
func (s *Service) publishAudit(ctx context.Context, action audit.Action) {
	if err := s.auditPublisher.Publish(ctx, action); err != nil {
		xlog.Error(ctx, "failed to publish audit action",
			xfield.String("action", fmt.Sprintf("%T", action)),
			xfield.Error(err),
		)
	}
}
