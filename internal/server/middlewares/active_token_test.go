package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	mock_middlewares "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/server/middlewares"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestRequireActiveToken(t *testing.T) {
	t.Parallel()

	activeClaims := &entity.AccessClaims{
		Email: "alice@example.com",
		Roles: []entity.Role{entity.RoleEditor},
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        xuuid.NewString(),
			Subject:   xuuid.NewString(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}

	tests := []struct {
		name           string
		authHeader     string
		prepareMock    func(*mock_middlewares.MockActiveTokenIntrospector)
		expectedStatus int
	}{
		{
			name:       "active token",
			authHeader: "Bearer valid",
			prepareMock: func(introspector *mock_middlewares.MockActiveTokenIntrospector) {
				introspector.EXPECT().
					Introspect(gomock.Any(), "valid").
					Return(activeClaims, nil)
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "missing token",
			authHeader:     "Bearer",
			prepareMock:    func(*mock_middlewares.MockActiveTokenIntrospector) {},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "malformed header",
			authHeader:     "Basic abc",
			prepareMock:    func(*mock_middlewares.MockActiveTokenIntrospector) {},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:       "inactive token",
			authHeader: "Bearer revoked",
			prepareMock: func(introspector *mock_middlewares.MockActiveTokenIntrospector) {
				introspector.EXPECT().
					Introspect(gomock.Any(), "revoked").
					Return(nil, apperr.ErrInvalidAccessToken)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:       "auth unavailable",
			authHeader: "Bearer valid",
			prepareMock: func(introspector *mock_middlewares.MockActiveTokenIntrospector) {
				introspector.EXPECT().
					Introspect(gomock.Any(), "valid").
					Return(nil, apperr.ErrAuthUnavailable)
			},
			expectedStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

			introspector := mock_middlewares.NewMockActiveTokenIntrospector(gomock.NewController(t))
			tt.prepareMock(introspector)

			c, rec := echotest.ContextConfig{
				Headers: map[string][]string{
					echo.HeaderContentType:   {echo.MIMEApplicationJSON},
					echo.HeaderAuthorization: {tt.authHeader},
				},
				Request: httptest.NewRequestWithContext(ctx, http.MethodPost, "/", http.NoBody),
			}.ToContextRecorder(t)

			mw := RequireActiveToken(introspector)

			nextFunc := func(c *echo.Context) error {
				return c.NoContent(http.StatusNoContent)
			}
			err := mw(nextFunc)(c)
			require.NoError(t, err)
			require.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}
