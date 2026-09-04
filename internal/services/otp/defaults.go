package otp

import (
	"time"

	"github.com/ruko1202/maintmode/internal/config"
)

// defaultTTL is the code lifetime when auth.otp_ttl is unset. Five minutes
// follows the sign-in design; the ticket said roughly ten, and the shorter
// window is the safer reading. The lifetime is not the only bound on guessing --
// defaultMaxAttempts caps a single code far more tightly -- but it is what bounds
// how long a code that leaked in transit stays worth anything, and it is also
// what bounds the reissue barrier claimSlot documents.
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

// defaultMaxAttempts is the guess ceiling when auth.otp_max_attempts is unset.
// Five is enough for a human mistyping six digits and nowhere near enough to
// search a million-value space.
const defaultMaxAttempts = 5

// maxAllowedAttempts caps whatever an operator configures. A ceiling that can be
// set arbitrarily high is not a ceiling, and attempts is a SMALLINT, so a large
// value is also an overflow waiting to happen — see MaxAttempts.
const maxAllowedAttempts = 10

// MaxAttempts returns the configured guess ceiling, guarded in both directions.
//
// A non-positive value falls back to the default rather than installing itself:
// config blocks carry no viper defaults, so an absent or half-filled auth block
// arrives as a bare Go zero, and a ceiling of zero refuses every guess. That is
// fail-closed inside machinery that otherwise fails open, which is the same trap
// the rate limiter's window options document.
//
// The clamp runs HERE, in int space, before the int16 conversion — the order is
// load-bearing rather than stylistic. attempts is a SMALLINT; a configured 40000
// converted first wraps to -25536, and the claim predicate `attempts < max` is
// then false for every row, so the endpoint refuses every guess. Clamping first
// makes any value that could wrap unreachable by the conversion.
//
// One owner, one read: this resolver is the only place the number is decided,
// and both consumers — the verify path's ceiling and the reissue barrier — read
// it from the same cached field rather than resolving their own copy. They
// enforce complementary halves of one rule (`attempts < max` when claiming,
// `attempts >= max` when barring), so a divergence would either hand out a sixth
// guess or leave the barrier permanently shut.
func MaxAttempts(cfg config.Auth) int16 {
	n := cfg.OTPMaxAttempts
	if n <= 0 {
		n = defaultMaxAttempts
	}
	if n > maxAllowedAttempts {
		n = maxAllowedAttempts
	}

	return int16(n)
}
