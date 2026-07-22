package httperrors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
)

// ToAPIError is the mapper the auth handlers actually call. This pins the
// signup-policy contract through that real path: 403 with the stable
// signup_disabled code and a fixed generic message — wrapped context (which
// could hint at invitation state) must never reach the response body. Wrapped
// variants must map identically.
func TestToAPIError_SignupDisabled(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
	}{
		{"sentinel", apperr.ErrSignupDisabled},
		{"wrapped", fmt.Errorf("get or create user: %w", apperr.ErrSignupDisabled)},
	}

	e := echo.New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			c := e.NewContext(httptest.NewRequest(http.MethodPost, "/", http.NoBody), rec)

			require.NoError(t, ToAPIError(c, "auth.exchange", tc.err))
			require.Equal(t, http.StatusForbidden, rec.Code)

			var resp ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			require.Equal(t, string(ErrSignupDisabled), resp.Code)
			require.Equal(t, "signup is disabled", resp.Message)
		})
	}
}
