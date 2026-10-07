package integration

import (
	"fmt"
	"net"
	"testing"

	"github.com/ruko1202/xhttp/dialguard"
	"github.com/stretchr/testify/require"
)

// A guard refusal arrives as a dial error, so it must be recognized before the
// generic "could not connect": the operator's remedy is a config key, not a
// network fix. The address the guard names stays out of the answer -- it is the
// internal address the caller was probing for.
func TestProbeFailureReason_NamesTheGuardNotTheAddress(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("email send: dial failed: %w", &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: fmt.Errorf("%w: 10.0.0.5:22", dialguard.ErrBlockedAddress),
	})

	reason := probeFailureReason(err)
	require.Contains(t, reason, "allow_internal_hosts")
	require.NotContains(t, reason, "10.0.0.5")
}
