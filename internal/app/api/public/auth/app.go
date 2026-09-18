package auth

import (
	"context"
	"time"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/services/auth"
	"github.com/ruko1202/maintmode/internal/services/authmethod"
	"github.com/ruko1202/maintmode/internal/services/otp"
	"github.com/ruko1202/maintmode/internal/services/token"
	"github.com/ruko1202/maintmode/internal/services/user"
)

type Implementation struct {
	// authMethods is the live login configuration. The sign-in button list is
	// read from its snapshot per request rather than from a config copy pinned
	// here, so a provider configured at runtime appears without a restart.
	authMethods *authmethod.Methods

	// authSettings answers which BUILT-IN methods this instance offers. Nil
	// lists NOTHING rather than everything: the sign-in gates refuse those same
	// methods, so listing them would advertise credentials that will not work.
	// Every binary that serves this endpoint wires it in bootstrap, so a nil is
	// a dropped wiring line rather than a configuration.
	authSettings AuthSettings

	authSrv  *auth.Service
	tokenSrv *token.Service
	userSrv  *user.Service
	otpSrv   *otp.Service

	// Backend-driven OAuth dance. These are zero when the dance is not
	// configured, in which case its routes are never registered (see the
	// config gate) and none of them is read.
	// danceCookiePath is the EXTERNAL cookie scope, taken from config rather
	// than from the mounted route — the proxy strips a prefix the handler never
	// sees. The config gate requires it, so it is never empty here.
	danceCookiePath      string
	frontendURL          string
	frontendCallbackPath string
	// otpResponseFloor is the minimum time RequestOTP takes to answer. It closes
	// a timing oracle rather than throttling anything; see acceptedOTPRequest.
	otpResponseFloor time.Duration
}

// defaultOTPResponseFloor is the floor when auth.otp_response_floor is unset.
// It has to sit above the issuance transaction, or the branch that does real
// work still stands out from the one that does not.
const defaultOTPResponseFloor = 300 * time.Millisecond

// otpResponseFloorFrom resolves the configured floor, falling back to the
// default. It lives here rather than beside the code TTL because it is an HTTP
// concern -- how long this handler takes to answer -- and this handler is its
// only consumer. The TTL is shared between the issuing service and the delivery
// processor, which is why that one is resolved in the domain package.
func otpResponseFloorFrom(cfg config.Auth) time.Duration {
	if cfg.OTPResponseFloor <= 0 {
		return defaultOTPResponseFloor
	}
	return cfg.OTPResponseFloor
}

func New(
	cfg config.Auth,
	authSrv *auth.Service,
	tokenSrv *token.Service,
	userSrv *user.Service,
	otpSrv *otp.Service,
) *Implementation {
	return &Implementation{
		authSrv:          authSrv,
		tokenSrv:         tokenSrv,
		userSrv:          userSrv,
		otpSrv:           otpSrv,
		otpResponseFloor: otpResponseFloorFrom(cfg),
	}
}

// AuthSettings reports whether a built-in sign-in method is currently offered.
//
// Declared consumer-side, one method wide: this layer needs the flag and
// nothing else, and the service that answers it also owns a guard and an audit
// trail that the listing has no business reaching.
type AuthSettings interface {
	// List is what the listing reads: one call for every built-in, rather than
	// one call per method on the login page's first request.
	List(ctx context.Context) ([]*entity.AuthMethodSetting, error)
	// Enabled answers a single method, which is the shape the sign-in gates
	// need -- each of them asks about exactly one.
	Enabled(ctx context.Context, method entity.AuthMethodName) (bool, error)
}

// WithAuthSettings attaches the built-in method flags the listing reads.
func (i *Implementation) WithAuthSettings(settings AuthSettings) *Implementation {
	i.authSettings = settings

	return i
}

// WithAuthMethods attaches the live login configuration, from which the
// providers endpoint reads its sign-in buttons.
//
// Separate from WithOAuthDance because the two are independent: an instance
// reachable only through the BFF path still needs its buttons, and still has no
// dance.
func (i *Implementation) WithAuthMethods(methods *authmethod.Methods) *Implementation {
	i.authMethods = methods

	return i
}

// WithOAuthDance attaches the backend-driven dance dependencies.
//
// It is a separate constructor step rather than more parameters on New because
// the dance is optional: an instance that configures no provider never
// registers these routes, and every existing caller of New keeps working
// unchanged.
//
// The state signature is NOT wired here: it belongs to the auth service, which
// derives it from the JWT issuer key it already holds — see WithDanceSigner.
//
// The cookie Secure flag is NOT captured here. It is aggregated over the
// providers that can dance, which are now added and removed at runtime, so it
// is read from the live snapshot per request -- see danceCookieSecure below and
// Snapshot.DanceCookieSecure for which way that aggregation leans and why.
func (i *Implementation) WithOAuthDance(appCfg config.App) *Implementation {
	i.danceCookiePath = appCfg.OAuthCookiePath
	i.frontendURL = appCfg.FrontendURL
	i.frontendCallbackPath = appCfg.OAuthCallbackPath

	return i
}

// danceCookieSecure reads the flag off the live snapshot.
//
// Fail-safe when no snapshot is wired at all: a cookie the browser withholds
// costs a sign-in, one it leaks over http costs the session.
func (i *Implementation) danceCookieSecure() bool {
	if i.authMethods == nil {
		return true
	}

	return i.authMethods.DanceCookieSecure()
}
