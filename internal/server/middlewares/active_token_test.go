package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	mock_middlewares "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/server/middlewares"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

func TestRequireActiveToken(t *testing.T) {
	t.Parallel()

	tokenRoles := []entity.Role{entity.RoleAdmin}
	storedRoles := []entity.Role{entity.RoleGuest}

	tests := []struct {
		name           string
		method         string
		authHeader     string
		withoutUser    bool
		prepareMock    func(*mock_middlewares.MockActiveTokenChecker)
		expectedStatus int
		// expectedRoles are the roles the next handler sees; nil when it must not run.
		expectedRoles []entity.Role
	}{
		{
			name:       "active token: next sees stored roles, not token roles",
			method:     http.MethodPost,
			authHeader: "Bearer valid",
			prepareMock: func(checker *mock_middlewares.MockActiveTokenChecker) {
				checker.EXPECT().
					EnsureActiveToken(gomock.Any(), "valid").
					Return(storedRoles, nil)
			},
			expectedStatus: http.StatusNoContent,
			expectedRoles:  storedRoles,
		},
		{
			name:           "safe method passes on token roles without a check",
			method:         http.MethodGet,
			authHeader:     "Bearer valid",
			prepareMock:    func(*mock_middlewares.MockActiveTokenChecker) {},
			expectedStatus: http.StatusNoContent,
			expectedRoles:  tokenRoles,
		},
		{
			name:           "missing token",
			method:         http.MethodPost,
			authHeader:     "Bearer",
			prepareMock:    func(*mock_middlewares.MockActiveTokenChecker) {},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "malformed header",
			method:         http.MethodPost,
			authHeader:     "Basic abc",
			prepareMock:    func(*mock_middlewares.MockActiveTokenChecker) {},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "no authenticated user in context",
			method:         http.MethodPost,
			authHeader:     "Bearer valid",
			withoutUser:    true,
			prepareMock:    func(*mock_middlewares.MockActiveTokenChecker) {},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:       "inactive token",
			method:     http.MethodDelete,
			authHeader: "Bearer revoked",
			prepareMock: func(checker *mock_middlewares.MockActiveTokenChecker) {
				checker.EXPECT().
					EnsureActiveToken(gomock.Any(), "revoked").
					Return(nil, apperr.ErrInvalidAccessToken)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:       "auth unavailable",
			method:     http.MethodPatch,
			authHeader: "Bearer valid",
			prepareMock: func(checker *mock_middlewares.MockActiveTokenChecker) {
				checker.EXPECT().
					EnsureActiveToken(gomock.Any(), "valid").
					Return(nil, apperr.ErrAuthUnavailable)
			},
			expectedStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

			checker := mock_middlewares.NewMockActiveTokenChecker(gomock.NewController(t))
			tt.prepareMock(checker)

			c, rec := echotest.ContextConfig{
				Headers: map[string][]string{
					echo.HeaderContentType:   {echo.MIMEApplicationJSON},
					echo.HeaderAuthorization: {tt.authHeader},
				},
				Request: httptest.NewRequestWithContext(ctx, tt.method, "/", http.NoBody),
			}.ToContextRecorder(t)
			tokenUser := &entity.User{ID: uuid.New(), Roles: tokenRoles}
			if !tt.withoutUser {
				xecho.UserToEchoCtx(c, tokenUser)
			}

			var seenRoles []entity.Role
			nextFunc := func(c *echo.Context) error {
				user, ok := xecho.UserFromEchoCtx(c)
				require.True(t, ok)
				seenRoles = user.Roles
				return c.NoContent(http.StatusNoContent)
			}
			err := RequireActiveToken(checker)(nextFunc)(c)
			require.NoError(t, err)
			require.Equal(t, tt.expectedStatus, rec.Code)
			require.Equal(t, tt.expectedRoles, seenRoles)
			// The verified token user is not mutated in place.
			require.Equal(t, tokenRoles, tokenUser.Roles)
		})
	}
}
