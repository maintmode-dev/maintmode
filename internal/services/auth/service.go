package auth

import (
	"context"
	"fmt"
	"time"

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
	// danceSigner and danceCodes are zero until WithDance is called, which only
	// happens when the backend-driven OAuth dance is configured. Every dance
	// route is behind the same config gate, so neither is ever reached unset.
	danceSigner   danceStateSigner
	danceCodes    DanceCodeStore
	danceGateway  DanceGateway
	danceStateTTL time.Duration
}

// DanceCodeStore is the one piece of Valkey the dance still needs: the token
// pair waiting behind a one-time code. Everything else the dance carries rides
// in signed cookies.
type DanceCodeStore interface {
	PutCode(ctx context.Context, code string, pair *entity.TokenPair) error
	ConsumeCode(ctx context.Context, code string) (*entity.TokenPair, error)
}

// DanceGateway is the provider side of the dance: where /start sends the
// browser, and where the callback redeems the code it comes back with.
//
// Both halves are one interface because both are the provider's contract rather
// than ours — the authorization URL carries the same client id, redirect URI and
// endpoint the exchange does, and splitting them would mean assembling that set
// twice.
type DanceGateway interface {
	AuthCodeURL(state, verifier string) string
	Exchange(ctx context.Context, code, codeVerifier string) (string, error)
}

// WithDance enables the backend-driven OAuth dance.
//
// It is a separate step rather than more constructor parameters because the
// dance is optional: an instance that configures no client_secret never
// registers its routes, and every existing caller of NewService keeps working
// unchanged.
//
// clientSecret is mixed into the signing key so that rotating EITHER it or the
// JWT issuer key ends every dance in flight — see newDanceStateSigner for why
// the two are combined through a KDF rather than concatenated.
func (s *Service) WithDance(
	authCfg config.Auth,
	clientSecret string,
	codes DanceCodeStore,
	gateway DanceGateway,
) *Service {
	s.danceSigner = newDanceStateSigner(s.cfg.PrivateKey, clientSecret)
	s.danceCodes = codes
	s.danceGateway = gateway
	s.danceStateTTL = danceStateTTL(authCfg)

	return s
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

// publishLoginFailure records a login that failed, with whatever attribution the
// attempt had.
//
// actor must never be nil: the audit renderer dereferences its fields without a
// guard, so a nil one panics the audit processor — asynchronously, long after
// the request that caused it returned cleanly. Pass the resolved user when there
// is one, &entity.User{Email: claimed} when an address was offered but matched
// nothing, and a bare &entity.User{} when the attempt carried no identity at
// all. The zero ID is the documented representation of "failed before the user
// was known", and such rows are found by their metadata instead: IP, user agent,
// failure reason.
func (s *Service) publishLoginFailure(
	ctx context.Context,
	actor *entity.User,
	meta *entity.AuditMetadata,
	reason entity.AuditFailureReason,
) {
	// Copied rather than written in place: callers reuse one metadata value
	// across several branches, and mutating it would leak one branch's reason
	// into another's record.
	failed := entity.AuditMetadata{FailureReason: reason}
	if meta != nil {
		failed = *meta
		failed.FailureReason = reason
	}

	s.publishAudit(ctx, audit.LoginFailed{User: actor, Meta: &failed})
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
