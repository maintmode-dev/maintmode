package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
)

// The floor falls back on any non-positive value, not just zero: a negative
// duration in config would otherwise skip the wait entirely and reopen the
// timing oracle the floor exists to close.
func TestOTPResponseFloorFrom(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		configured time.Duration
		want       time.Duration
	}{
		"configured": {time.Second, time.Second},
		"zero":       {0, defaultOTPResponseFloor},
		"negative":   {-time.Second, defaultOTPResponseFloor},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, otpResponseFloorFrom(config.Auth{OTPResponseFloor: tc.configured}))
		})
	}
}
