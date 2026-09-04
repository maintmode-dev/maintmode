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
