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
	"github.com/ruko1202/maintmode/internal/utils/xecho"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestRequireAccessToken(t *testing.T) {
	t.Parallel()

	user := &entity.User{
		ID:    xuuid.New(),
		Roles: []entity.Role{entity.RoleEditor},
		Email: "alice@example.com",
	}

	tests := []struct {
		name           string
		authHeader     string
		prepareMock    func(*mock_middlewares.MockTokenVerifier, *entity.AccessClaims)
		expectedStatus int
		expectUser     *entity.User
		mutateClaims   func(claims *entity.AccessClaims)
	}{
		{
			name:         "valid token",
			authHeader:   "Bearer valid",
			mutateClaims: func(*entity.AccessClaims) {},
			prepareMock: func(verifier *mock_middlewares.MockTokenVerifier, claims *entity.AccessClaims) {
				verifier.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Return(claims, nil)
			},
			expectedStatus: http.StatusNoContent,
			expectUser:     user,
		}, {
			name:           "missing token",
			authHeader:     "Bearer",
			mutateClaims:   func(*entity.AccessClaims) {},
			prepareMock:    func(*mock_middlewares.MockTokenVerifier, *entity.AccessClaims) {},
			expectedStatus: http.StatusUnauthorized,
		}, {
			name:           "malformed token",
			authHeader:     "Basic abc",
			mutateClaims:   func(*entity.AccessClaims) {},
			prepareMock:    func(*mock_middlewares.MockTokenVerifier, *entity.AccessClaims) {},
			expectedStatus: http.StatusUnauthorized,
		}, {
			name:         "invalid token",
			authHeader:   "Bearer invalid",
			mutateClaims: func(*entity.AccessClaims) {},
			prepareMock: func(verifier *mock_middlewares.MockTokenVerifier, _ *entity.AccessClaims) {
				verifier.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Return(nil, apperr.ErrInvalidAccessToken)
			},
			expectedStatus: http.StatusUnauthorized,
		}, {
			name:         "auth unavailable",
			authHeader:   "Bearer valid",
			mutateClaims: func(*entity.AccessClaims) {},
			prepareMock: func(verifier *mock_middlewares.MockTokenVerifier, _ *entity.AccessClaims) {
				verifier.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Return(nil, apperr.ErrAuthUnavailable)
			},
			expectedStatus: http.StatusServiceUnavailable,
		}, {
			name:         "invalid subject claim",
			authHeader:   "Bearer valid",
			mutateClaims: func(claims *entity.AccessClaims) { claims.Subject = "not-a-uuid" },
			prepareMock: func(verifier *mock_middlewares.MockTokenVerifier, claims *entity.AccessClaims) {
				verifier.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Return(claims, nil)
			},
			expectedStatus: http.StatusUnauthorized,
		}, {
			name:         "missing email claim",
			authHeader:   "Bearer valid",
			mutateClaims: func(claims *entity.AccessClaims) { claims.Email = "" },
			prepareMock: func(verifier *mock_middlewares.MockTokenVerifier, claims *entity.AccessClaims) {
				verifier.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Return(claims, nil)
			},
			expectedStatus: http.StatusUnauthorized,
		}, {
			name:         "missing roles claim",
			authHeader:   "Bearer valid",
			mutateClaims: func(claims *entity.AccessClaims) { claims.Roles = nil },
			prepareMock: func(verifier *mock_middlewares.MockTokenVerifier, claims *entity.AccessClaims) {
				verifier.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Return(claims, nil)
			},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

			claims := &entity.AccessClaims{
				Email: user.Email,
				Roles: user.Roles,
				RegisteredClaims: jwt.RegisteredClaims{
					ID:        xuuid.NewString(),
					Subject:   user.ID.String(),
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				},
			}
			tt.mutateClaims(claims)

			verifier := mock_middlewares.NewMockTokenVerifier(gomock.NewController(t))
			tt.prepareMock(verifier, claims)

			c, rec := echotest.ContextConfig{
				Headers: map[string][]string{
					echo.HeaderContentType:   {echo.MIMEApplicationJSON},
					echo.HeaderAuthorization: {tt.authHeader},
				},
				Request: httptest.NewRequestWithContext(ctx, http.MethodGet, "/", http.NoBody),
			}.ToContextRecorder(t)

			mw := RequireAccessToken(verifier)

			nextFunc := func(c *echo.Context) error {
				actualUser, ok := xecho.UserFromEchoCtx(c)
				require.Equal(t, tt.expectUser != nil, ok)
				if tt.expectUser != nil {
					require.Equal(t, user, actualUser)
				}
				return c.NoContent(http.StatusNoContent)
			}
			err := mw(nextFunc)(c)
			require.NoError(t, err)
			require.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}
