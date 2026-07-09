package httperrors

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
)

// TestMapError_IntegrationSentinels pins the HTTP status mapping for the
// integration registry domain errors (RUK-196): not-found -> 404, conflict -> 409,
// unknown-kind (wraps ErrValidation) -> 400. Wrapped variants must map the same.
func TestMapError_IntegrationSentinels(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"not found", apperr.ErrIntegrationNotFound, http.StatusNotFound},
		{"not found wrapped", fmt.Errorf("resolve %q: %w", "slack", apperr.ErrIntegrationNotFound), http.StatusNotFound},
		{"conflict", apperr.ErrIntegrationConflict, http.StatusConflict},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			status, resp := mapError(tc.err)
			require.Equal(t, tc.wantStatus, status)
			require.NotNil(t, resp)
		})
	}
}
