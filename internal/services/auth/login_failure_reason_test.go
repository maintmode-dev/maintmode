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
