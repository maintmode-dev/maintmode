package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// doChangePassword drives the real handler as the given user.
func doChangePassword(t *testing.T, impl *Implementation, user *entity.User, body string) recordedResponse {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/password", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	c := echotest.ContextConfig{Request: req, Response: rec}.ToContext(t)
	xecho.UserToEchoCtx(c, user)
	require.NoError(t, impl.ChangePassword(c))

	return recordedResponse{
		status:       rec.Code,
		body:         rec.Body.String(),
		cacheControl: rec.Header().Get(echo.HeaderCacheControl),
	}
}

// The status codes a client classifies on. Unlike the login and reset routes,
// this one is authenticated and the caller owns the account, so the failures
// are told apart on purpose -- a form has to know whether to say "wrong
// password" or "too short", and a BFF has to know whether to bounce the
// operator to sign-in.
//
// Two of these used to be 500. ErrPasswordPolicy was not wrapped in
// ErrValidation, and ErrInvalidCredentials was missing from the auth mapper
// entirely, so both fell through to the default arm and returned an internal
// error a client could not act on.
func TestChangePassword_StatusesAreDistinguishable(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	impl := initImpl(t)

	newUser := func(t *testing.T) *entity.User {
		t.Helper()

		u, err := impl.userSrv.GetOrCreateByAuthInfo(ctx, entity.AuthMethodGoogle, &entity.OAuthProviderUserInfo{
			ID:    "change-pw-" + uuid.NewString(),
			Email: uuid.NewString() + "@test.local",
			Name:  "Change Password User",
		}, entity.UserCreationPolicy{AllowCreate: true})
		require.NoError(t, err)

		return u
	}

	t.Run("a wrong current password is 401", func(t *testing.T) {
		t.Parallel()

		user := newUser(t)
		require.NoError(t, impl.authSrv.ChangePassword(ctx, &entity.ChangePasswordCmd{
			UserID: user.ID, NewPassword: "the-original-password", ClientIP: "10.0.0.1",
		}))

		got := doChangePassword(t, impl, user,
			`{"current_password":"not-the-original","new_password":"a-new-long-password"}`)
		require.Equal(t, http.StatusUnauthorized, got.status)
	})

	t.Run("a password below the length floor is 400", func(t *testing.T) {
		t.Parallel()

		got := doChangePassword(t, impl, newUser(t), `{"new_password":"short"}`)
		require.Equal(t, http.StatusBadRequest, got.status,
			"a client must be able to tell the user their password was too short")
	})

	t.Run("setting a first password is 204", func(t *testing.T) {
		t.Parallel()

		got := doChangePassword(t, impl, newUser(t), `{"new_password":"a-perfectly-long-password"}`)
		require.Equal(t, http.StatusNoContent, got.status)
		require.Empty(t, got.body)
	})
}
