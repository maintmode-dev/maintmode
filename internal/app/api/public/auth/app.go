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

	// authSettings answers which BUILT-IN methods this instance offers.
	authSettings AuthSettings

	authSrv  *auth.Service
	tokenSrv *token.Service
	userSrv  *user.Service
	otpSrv   *otp.Service

	// Backend-driven OAuth dance. These are empty when the frontend half of the
	// dance is not configured, in which case /start refuses (see the config
	// gate in oauth_start.go).
	// danceCookiePath is the EXTERNAL cookie scope, taken from config rather
	// than from the mounted route — the proxy strips a prefix the handler never
	// sees. The config gate requires it, so it is never empty past the gate.
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

// New builds the auth handlers.
//
// authMethods is the live login configuration the sign-in button list is read
// from, and authSettings the built-in method flags; the two are independent,
// since an instance reachable only through the BFF path still needs its
// buttons.
//
// The dance's cookie scope and frontend addresses come from appCfg. The state
// signature is NOT wired here: it belongs to the auth service, which derives it
// from the JWT issuer key it already holds. Nor is the cookie Secure flag: it is
// aggregated over the providers that can dance, which are added and removed at
// runtime, so it is read from the live snapshot per request -- see danceCookie
// and Snapshot.DanceCookieSecure for which way that aggregation leans and why.
func New(
	cfg config.Auth,
	authSrv *auth.Service,
	tokenSrv *token.Service,
	userSrv *user.Service,
	otpSrv *otp.Service,
	authMethods *authmethod.Methods,
	authSettings AuthSettings,
	appCfg config.App,
) *Implementation {
	return &Implementation{
		authMethods:          authMethods,
		authSettings:         authSettings,
		authSrv:              authSrv,
		tokenSrv:             tokenSrv,
		userSrv:              userSrv,
		otpSrv:               otpSrv,
		danceCookiePath:      appCfg.OAuthCookiePath,
		frontendURL:          appCfg.FrontendURL,
		frontendCallbackPath: appCfg.OAuthCallbackPath,
		otpResponseFloor:     otpResponseFloorFrom(cfg),
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
