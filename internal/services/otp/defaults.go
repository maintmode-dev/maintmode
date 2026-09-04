package otp

import (
	"time"

	"github.com/ruko1202/maintmode/internal/config"
)

// defaultTTL is the code lifetime when auth.otp_ttl is unset. Five minutes
// follows the sign-in design; the ticket said roughly ten, and the shorter
// window is the safer reading while no per-code attempt ceiling exists.
const defaultTTL = 5 * time.Minute

// TTL returns the configured code lifetime, or the default when unset.
//
// Both the issuing service and the delivery processor need this number -- one to
// stamp expires_at, the other to write "expires in N minutes" in the email -- and
// they must not disagree, or the copy contradicts the row. Hence one resolver
// rather than a fallback repeated at each wiring site.
func TTL(cfg config.Auth) time.Duration {
	if cfg.OTPTTL <= 0 {
		return defaultTTL
	}
	return cfg.OTPTTL
}
