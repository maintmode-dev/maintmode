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
	// invitations is zero until WithInvitations is called. A nil claimer means
	// this instance completes no invited dances: every handle is refused, which
	// is the fail-closed direction — never a panic, and never a dance that
	// creates an account because the claimer was missing.
	invitations InvitationClaimer
}

// DanceCodeStore is the Valkey the dance needs: the token pair waiting behind a
// one-time code, and — for an invited dance — the invitation behind an opaque
// handle. Everything else the dance carries rides in signed cookies.
//
// The invitation half is stored rather than signed into the state because the
// state travels to the provider in the clear, and the invitation token is a
// bearer credential with a multi-day life. Only the handle reaches the browser;
// only the id reaches Valkey.
type DanceCodeStore interface {
	PutCode(ctx context.Context, code string, pair *entity.TokenPair) error
	ConsumeCode(ctx context.Context, code string) (*entity.TokenPair, error)
	PutInvitationHandle(ctx context.Context, handle string, invitationID uuid.UUID) error
	// ConsumeInvitationHandle returns nil with no error when there is nothing to
	// redeem. That is the ordinary path for a dead or forged invitation, not a
	// fault: /start stores a handle whenever the parameter is present, without
	// first resolving the token, which is what keeps it from being an oracle.
	ConsumeInvitationHandle(ctx context.Context, handle string) (*uuid.UUID, error)
}

// InvitationClaimer is the invitation side of an invited dance.
//
// TWO operations, because the guard and the claim sit on opposite sides of user
// creation: the email match must refuse BEFORE any account exists, while the
// roles can only be granted AFTER the user has an id. One combined call cannot
// be in both places.
//
// It is an interface here, on the consumer side, rather than a direct
// dependency on the invitation service, because that dependency cannot exist:
// invitation.NewService already takes auth.Service as its TokenIssuer, so an
// import in this direction closes a cycle the compiler enforces. WithInvitations
// wires the real implementation after both services are constructed.
type InvitationClaimer interface {
	// PrepareHandle runs at /start. It parks the invitation the raw token names
	// behind the opaque handle, so the token itself never travels further.
	//
	// It does NOT report whether the token names a live invitation, deliberately:
	// answering that at /start would make it an oracle for guessing tokens before
	// authenticating with anything. A token that names nothing yields a handle
	// that redeems to nothing in phase 0, after the provider round trip.
	PrepareHandle(ctx context.Context, handle, invitationToken string) error
	// ResolveForIdentity runs BEFORE sign-in. It redeems the handle, checks the
	// invitation is live, and enforces the email match against the verified
	// provider claims. It grants nothing and writes nothing beyond consuming the
	// handle.
	//
	// A nil result with a nil error is not possible: absence is reported as an
	// error, never as an empty value. An invitation may legitimately carry no
	// roles, so a nil-or-empty slice must never be readable as "no invitation" —
	// that overloading is what decides AllowCreate, and on a zero-admin instance
	// a wrong answer grants admin.
	ResolveForIdentity(ctx context.Context, handle string, claims *entity.OAuthIDTokenClaims) (*entity.ResolvedInvitation, error)
	// ClaimForUser runs AFTER the user exists. It flips pending→accepted and
	// assigns the invitation's roles in ONE transaction, so an accepted
	// invitation never leaves a user without its roles.
	ClaimForUser(ctx context.Context, inv *entity.ResolvedInvitation, userID uuid.UUID) error
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
	// config.Auth owns the fallback so the wiring, which must give the
	// invitation-handle store the SAME lifetime, resolves it from one place. Two
	// copies would drift the instant one was tuned, and the symptom — handles
	// expiring mid-consent while states stayed valid — reads as a flaky provider
	// rather than a config bug.
	s.danceStateTTL = authCfg.DanceStateTTL()

	return s
}

// WithInvitations enables invited dances.
//
// A separate step from WithDance, and from the constructor, for a reason the
// compiler enforces rather than a stylistic one: invitation.NewService takes
// this service as its TokenIssuer, so the invitation service cannot exist when
// this one is built. The wiring calls this once both do.
//
// Leaving it unset is safe and is the fail-closed default: CompleteDance
// refuses every handle it is given, so an instance that forgot this call
// declines invited sign-ins rather than completing them unguarded.
func (s *Service) WithInvitations(claimer InvitationClaimer) *Service {
	s.invitations = claimer

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
