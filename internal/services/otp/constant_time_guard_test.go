package otp_test

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestVerifyComparesSecretsInConstantTime is a source-level guard, and it is a
// source-level guard on purpose.
//
// The property is "the comparison does not return early on the first differing
// byte", which no black-box test can observe: a == comparison passes every
// behavioral test in this file. Timing assertions in Go tests are too flaky to
// stand in for it. So the check is mechanical instead -- it reads the verify
// source and fails if either secret is ever compared with an operator.
//
// Without this, a later refactor that "simplifies" subtle.ConstantTimeCompare
// into == leaks the shared prefix length of a code or a nonce through response
// timing, with every other test still green.
func TestVerifyComparesSecretsInConstantTime(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("verify.go")
	require.NoError(t, err)

	require.Contains(t, string(src), "subtle.ConstantTimeCompare",
		"the verify path must compare secrets in constant time")

	// Catches `cred.SecretHash ==`, `== cmd.Code`, `*cred.SessionNonce !=` and
	// the like, while leaving comparisons against nil or a sentinel alone.
	banned := regexp.MustCompile(`(?:SecretHash|SessionNonce|cmd\.Code)\s*[!=]=|[!=]=\s*(?:cmd\.Code|cmd\.SessionNonce)`)
	require.NotRegexp(t, banned, string(src),
		"compare secrets with subtle.ConstantTimeCompare, never with an operator")
}
