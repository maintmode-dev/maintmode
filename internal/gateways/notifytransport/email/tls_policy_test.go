package emailtransport

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wneessen/go-mail"
)

// TestTLSPolicy pins the config-string -> TLS-posture mapping. The default
// (unknown/empty) must be TLSMandatory — a safe default that keeps STARTTLS on
// unless an operator explicitly opts down.
func TestTLSPolicy(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want mail.TLSPolicy
	}{
		{tlsPolicyNone, mail.NoTLS},
		{tlsPolicyOpportunistic, mail.TLSOpportunistic},
		{"mandatory", mail.TLSMandatory},
		{"", mail.TLSMandatory},          // empty defaults to mandatory
		{"bogus", mail.TLSMandatory},     // unknown defaults to mandatory
		{"MANDATORY", mail.TLSMandatory}, // case-sensitive; anything non-matching defaults safe
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tlsPolicy(tc.in))
		})
	}
}
