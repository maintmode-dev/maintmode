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
