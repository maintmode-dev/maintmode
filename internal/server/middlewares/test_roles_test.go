package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

func TestParseTestRoles(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		raw     string
		want    []entity.Role
		wantErr error
	}{
		{name: "single role", raw: "admin", want: []entity.Role{entity.RoleAdmin}},
		{name: "multiple roles", raw: "admin,editor", want: []entity.Role{entity.RoleAdmin, entity.RoleEditor}},
		{name: "spaces trimmed", raw: " admin ,  reviewer ", want: []entity.Role{entity.RoleAdmin, entity.RoleReviewer}},
		{name: "case insensitive", raw: "Admin,EDITOR", want: []entity.Role{entity.RoleAdmin, entity.RoleEditor}},
		{name: "duplicates collapsed", raw: "admin,admin, Admin ,editor", want: []entity.Role{entity.RoleAdmin, entity.RoleEditor}},
		{name: "empty value equals absent header", raw: "", want: nil},
		{name: "blank value equals absent header", raw: "   ", want: nil},
		{name: "empty items skipped", raw: "admin,,editor,", want: []entity.Role{entity.RoleAdmin, entity.RoleEditor}},
		{name: "unknown role fails loudly", raw: "admin,superuser", wantErr: apperr.ErrInvalidRole},
		{name: "system role is not requestable", raw: "system admin", wantErr: apperr.ErrInvalidRole},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			roles, err := parseTestRoles(t.Context(), tc.raw)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Nil(t, roles)
				return
			}

			require.NoError(t, err)
			if tc.want == nil {
				require.Empty(t, roles)
				return
			}
			require.Equal(t, tc.want, roles)
		})
	}
}

func TestInjectTestRoles(t *testing.T) {
	t.Parallel()

	next := func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) }

	t.Run("valid header lands roles in context", func(t *testing.T) {
		t.Parallel()
		c, rec := newTestRolesContext(t, "admin,reviewer")

		require.NoError(t, InjectTestRoles()(next)(c))
		require.Equal(t, http.StatusNoContent, rec.Code)

		roles, ok := xecho.TestRolesFromEchoCtx(c)
		require.True(t, ok)
		require.Equal(t, []entity.Role{entity.RoleAdmin, entity.RoleReviewer}, roles)
	})

	t.Run("absent header is a no-op", func(t *testing.T) {
		t.Parallel()
		c, rec := newTestRolesContext(t, "")

		require.NoError(t, InjectTestRoles()(next)(c))
		require.Equal(t, http.StatusNoContent, rec.Code)

		_, ok := xecho.TestRolesFromEchoCtx(c)
		require.False(t, ok)
	})

	t.Run("unknown role fails with 400", func(t *testing.T) {
		t.Parallel()
		c, rec := newTestRolesContext(t, "admin,superuser")

		// The middleware routes the error through httperrors, yielding 400.
		require.NoError(t, InjectTestRoles()(next)(c))
		require.Equal(t, http.StatusBadRequest, rec.Code)

		_, ok := xecho.TestRolesFromEchoCtx(c)
		require.False(t, ok)
	})
}

func newTestRolesContext(t *testing.T, header string) (*echo.Context, *httptest.ResponseRecorder) {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/auth/exchange/google", http.NoBody)
	if header != "" {
		request.Header.Set(headerTestRoles, header)
	}
	return echotest.ContextConfig{Request: request}.ToContextRecorder(t)
}
