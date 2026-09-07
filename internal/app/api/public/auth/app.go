package auth

import (
	"time"

	"github.com/ruko1202/maintmode/internal/config"

	"github.com/ruko1202/maintmode/internal/services/auth"
	"github.com/ruko1202/maintmode/internal/services/otp"
	"github.com/ruko1202/maintmode/internal/services/token"
	"github.com/ruko1202/maintmode/internal/services/user"
)

type Implementation struct {
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
	danceCookiePath string
	// danceCookieSecure follows the redirect_uri's scheme, not the environment
	// name — see the cookie builder for why the environment name was wrong.
	danceCookieSecure    bool
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

// WithOAuthDance attaches the backend-driven dance dependencies.
//
// It is a separate constructor step rather than more parameters on New because
// the dance is optional: an instance that configures no client_secret never
// registers these routes, and every existing caller of New keeps working
// unchanged.
//
// The state signature is NOT wired here: it belongs to the auth service, which
// derives it from the JWT issuer key it already holds — see WithDanceSigner.
func (i *Implementation) WithOAuthDance(
	googleCfg config.GoogleOauthProvider,
	appCfg config.App,
) *Implementation {
	i.danceCookiePath = appCfg.OAuthCookiePath
	i.danceCookieSecure = danceCookieSecure(googleCfg.RedirectURI)
	i.frontendURL = appCfg.FrontendURL
	i.frontendCallbackPath = appCfg.OAuthCallbackPath

	return i
}
