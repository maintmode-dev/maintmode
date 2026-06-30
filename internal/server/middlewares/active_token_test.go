package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	mock_middlewares "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/server/middlewares"
)

func TestRequireActiveToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		authHeader     string
		prepareMock    func(*mock_middlewares.MockActiveTokenChecker)
		expectedStatus int
	}{
		{
			name:       "active token",
			authHeader: "Bearer valid",
			prepareMock: func(checker *mock_middlewares.MockActiveTokenChecker) {
				checker.EXPECT().
					EnsureActiveToken(gomock.Any(), "valid").
					Return(nil)
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "missing token",
			authHeader:     "Bearer",
			prepareMock:    func(*mock_middlewares.MockActiveTokenChecker) {},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "malformed header",
			authHeader:     "Basic abc",
			prepareMock:    func(*mock_middlewares.MockActiveTokenChecker) {},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:       "inactive token",
			authHeader: "Bearer revoked",
			prepareMock: func(checker *mock_middlewares.MockActiveTokenChecker) {
				checker.EXPECT().
					EnsureActiveToken(gomock.Any(), "revoked").
					Return(apperr.ErrInvalidAccessToken)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:       "auth unavailable",
			authHeader: "Bearer valid",
			prepareMock: func(checker *mock_middlewares.MockActiveTokenChecker) {
				checker.EXPECT().
					EnsureActiveToken(gomock.Any(), "valid").
					Return(apperr.ErrAuthUnavailable)
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
				Request: httptest.NewRequestWithContext(ctx, http.MethodPost, "/", http.NoBody),
			}.ToContextRecorder(t)

			mw := RequireActiveToken(checker)

			nextFunc := func(c *echo.Context) error {
				return c.NoContent(http.StatusNoContent)
			}
			err := mw(nextFunc)(c)
			require.NoError(t, err)
			require.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}
