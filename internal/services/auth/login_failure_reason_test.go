package auth

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestProvisioningFailureReason(t *testing.T) {
	t.Parallel()

	t.Run("signup refusal maps to the dedicated reason, even wrapped", func(t *testing.T) {
		t.Parallel()
		err := fmt.Errorf("get or create user: %w", apperr.ErrSignupDisabled)
		require.Equal(t, entity.AuditFailureSignupDisabled, provisioningFailureReason(err))
	})

	t.Run("any other failure stays generic provisioning", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, entity.AuditFailureUserProvisioning, provisioningFailureReason(errors.New("db down")))
	})
}

// TestIssuanceFailureReason pins the distinction an operator reads the audit
// trail for.
//
// Both branches end the same way for the user — no token — so nothing in the
// response tells them apart. Collapsing this mapper to a constant leaves every
// handler test green while a run of ordinary blocked-user sign-ins becomes
// indistinguishable from a token service that has stopped working.
func TestIssuanceFailureReason(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err  error
		want entity.AuditFailureReason
	}{
		"blocked user is not an incident": {
			err:  apperr.ErrUserBlocked,
			want: entity.AuditFailureUserBlocked,
		},
		// IssueTokenPair wraps, so the sentinel arrives buried.
		"blocked user survives wrapping": {
			err:  fmt.Errorf("issue access token: %w", apperr.ErrUserBlocked),
			want: entity.AuditFailureUserBlocked,
		},
		"anything else is a genuine issuance failure": {
			err:  errors.New("database is down"),
			want: entity.AuditFailureTokenIssuance,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, issuanceFailureReason(tt.err))
		})
	}
}
