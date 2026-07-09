package httperrors

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
)

// ToAPIError is the mapper the integration handlers actually call. This pins the
// end-to-end status for the integration sentinels through that real path — in
// particular ErrUnknownIntegrationKind, which wraps ErrValidation and so maps to
// 400 via the generic ErrValidation arm (not the not-found/conflict arm that
// delegates to mapError). Wrapped variants must map identically.
func TestToAPIError_IntegrationSentinels(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"unknown kind", apperr.ErrUnknownIntegrationKind, http.StatusBadRequest},
		{"unknown kind wrapped", fmt.Errorf("parse: %w", apperr.ErrUnknownIntegrationKind), http.StatusBadRequest},
		{"not found", apperr.ErrIntegrationNotFound, http.StatusNotFound},
		{"conflict", apperr.ErrIntegrationConflict, http.StatusConflict},
	}

	e := echo.New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			c := e.NewContext(httptest.NewRequest(http.MethodPost, "/", http.NoBody), rec)

			require.NoError(t, ToAPIError(c, "integration.create", tc.err))
			require.Equal(t, tc.wantStatus, rec.Code)
		})
	}
}
