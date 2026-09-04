package otp_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/services/otp"
)

// Both knobs fall back on a non-positive value rather than only on zero: a
// negative duration in config would otherwise stamp an expires_at in the past,
// issuing codes that are dead on arrival, or skip the response floor entirely
// and reopen the timing oracle.
func TestTTL(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		configured time.Duration
		want       time.Duration
	}{
		"configured": {2 * time.Minute, 2 * time.Minute},
		"zero":       {0, 5 * time.Minute},
		"negative":   {-time.Minute, 5 * time.Minute},
		"sub-second": {500 * time.Millisecond, 500 * time.Millisecond},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, otp.TTL(config.Auth{OTPTTL: tc.configured}))
		})
	}
}

// TestMaxAttempts pins both guards and, more importantly, the ORDER they are
// applied in.
//
// The clamp runs in int space, before the int16 conversion, and the "wraps
// without the clamp" case is why. attempts is a SMALLINT, so a configured 40000
// converted first becomes -25536; the ceiling predicate `attempts < max` is then
// false for every row and the endpoint refuses every guess -- fail-closed, in
// machinery whose other guard exists to avoid exactly that. Clamping first makes
// the conversion unreachable for any value that could wrap.
func TestMaxAttempts(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		configured int
		want       int16
	}{
		"configured":            {3, 3},
		"zero":                  {0, 5},
		"negative":              {-1, 5},
		"at the clamp":          {10, 10},
		"above the clamp":       {11, 10},
		"wraps without a clamp": {40000, 10},
		"int16 overflow":        {70000, 10},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, otp.MaxAttempts(config.Auth{OTPMaxAttempts: tc.configured}))
		})
	}
}
