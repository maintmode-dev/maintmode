package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

// TestValidateVerifyOTPCmd tests the validator DIRECTLY, and that is the point.
//
// The indistinguishability test lists "oversized email" and "non-digit code"
// among its cases, but its only assertion is that every case answers an
// identical 401 -- and an request that skipped validation entirely also answers
// 401, by falling through to the service and being refused there. So those case
// names promise the rules are enforced while the assertions cannot see them:
// deleting the whole validation call left the package green.
//
// The rules are not cosmetic. The address reaches audit_log.actor and
// entity_id, both carrying btree indexes that error above ~2704 bytes, on an
// endpoint any unauthenticated caller can reach. The code length keeps a
// constant-time comparison from being handed a value whose size alone is
// informative.
func TestValidateVerifyOTPCmd(t *testing.T) {
	t.Parallel()

	valid := func() *entity.VerifyOTPCmd {
		return &entity.VerifyOTPCmd{
			Email:        "user@example.com",
			Code:         "123456",
			SessionNonce: strings.Repeat("A", 43) + "=",
			ClientIP:     "203.0.113.1",
		}
	}

	require.NoError(t, validateVerifyOTPCmd(t.Context(), valid()),
		"an ordinary request must pass, or the endpoint is unusable")

	for name, mutate := range map[string]func(*entity.VerifyOTPCmd){
		"absent email":        func(c *entity.VerifyOTPCmd) { c.Email = "" },
		"malformed email":     func(c *entity.VerifyOTPCmd) { c.Email = "not-an-address" },
		"oversized email":     func(c *entity.VerifyOTPCmd) { c.Email = strings.Repeat("a", 300) + "@example.com" },
		"non-canonical email": func(c *entity.VerifyOTPCmd) { c.Email = "user@example.com\u200b" },
		"absent code":         func(c *entity.VerifyOTPCmd) { c.Code = "" },
		"short code":          func(c *entity.VerifyOTPCmd) { c.Code = "12345" },
		"long code":           func(c *entity.VerifyOTPCmd) { c.Code = "1234567" },
		"non-digit code":      func(c *entity.VerifyOTPCmd) { c.Code = "12345a" },
		"absent nonce":        func(c *entity.VerifyOTPCmd) { c.SessionNonce = "" },
		"oversized nonce":     func(c *entity.VerifyOTPCmd) { c.SessionNonce = strings.Repeat("n", maxSessionNonceLen+1) },
		"absent client ip":    func(c *entity.VerifyOTPCmd) { c.ClientIP = "" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := valid()
			mutate(cmd)

			require.Error(t, validateVerifyOTPCmd(t.Context(), cmd),
				"%s must be refused before it reaches the service", name)
		})
	}
}

// TestValidateRequestOTP is the same guarantee for the sibling endpoint. It
// shares the per-address limiter, so it must reject the same addresses -- a
// variant accepted here but refused there would split a victim's budget.
func TestValidateRequestOTP(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateRequestOTP(t.Context(),
		&apiauthmodels.RequestOTPRequest{Email: "user@example.com"}))

	for name, email := range map[string]string{
		"absent":        "",
		"malformed":     "not-an-address",
		"oversized":     strings.Repeat("a", 300) + "@example.com",
		"non-canonical": "user@example.com\u200b",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.Error(t, validateRequestOTP(t.Context(),
				&apiauthmodels.RequestOTPRequest{Email: email}),
				"%s must be refused", name)
		})
	}
}
