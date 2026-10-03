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
	"github.com/ruko1202/maintmode/internal/utils/xcripto"
)

// Pins the outcomes of POST /users/invitations/accept/password as the frontend
// branches on them: each refusal reaches the client as its own status and code,
// wrapped the way the service wraps it. The codes are literals on purpose -- a
// renamed constant must fail here rather than silently change the contract.
func TestToAPIError_AcceptInvitationWithPassword(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "no live invitation",
			err:        fmt.Errorf("resolve invitation: %w", apperr.ErrInvalidInvitation),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid",
		},
		{
			name:       "password outside the policy",
			err:        fmt.Errorf("%w: %w", apperr.ErrValidation, xcripto.ErrPasswordPolicy),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid request",
		},
		{
			name:       "email_password disabled",
			err:        apperr.ErrSignInMethodDisabled,
			wantStatus: http.StatusForbidden,
			wantCode:   "method_disabled",
		},
		{
			name:       "an account with the email already exists",
			err:        fmt.Errorf("create user: %w", apperr.ErrUserAlreadyExists),
			wantStatus: http.StatusConflict,
			wantCode:   "conflict",
		},
	}

	e := echo.New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			c := e.NewContext(httptest.NewRequest(http.MethodPost, "/", http.NoBody), rec)

			require.NoError(t, ToAPIError(c, "api.Auth.AcceptInvitationWithPassword", tc.err))
			require.Equal(t, tc.wantStatus, rec.Code)

			var resp ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			require.Equal(t, tc.wantCode, resp.Code)
		})
	}
}
