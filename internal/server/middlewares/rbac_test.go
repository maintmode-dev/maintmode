package middlewares

import (
	"context"
	"fmt"
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

	"github.com/ruko1202/maintmode/internal/entity"
	mock_middlewares "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/server/middlewares"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

func TestRequireScenario(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		allowed        bool
		authzErr       error
		expectedStatus int
	}{
		{
			name:           "allowed",
			allowed:        true,
			expectedStatus: http.StatusNoContent,
		}, {
			name:           "forbidden",
			allowed:        false,
			authzErr:       nil,
			expectedStatus: http.StatusForbidden,
		}, {
			name:           "authorizer error",
			allowed:        false,
			authzErr:       fmt.Errorf("some err"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

			c, rec := echotest.ContextConfig{
				Headers: map[string][]string{
					echo.HeaderContentType: {echo.MIMEApplicationJSON},
				},
				Request: httptest.NewRequestWithContext(ctx, http.MethodGet, "/", http.NoBody),
			}.ToContextRecorder(t)

			xecho.UserToEchoCtx(c, &entity.User{
				ID:    uuid.New(),
				Email: "alice@example.com",
				Roles: []entity.Role{entity.RoleEditor},
			})

			authorizer := mock_middlewares.NewMockAuthorizer(gomock.NewController(t))
			authorizer.EXPECT().
				Allow(gomock.Any(), []entity.Role{entity.RoleEditor}, entity.AuthzScenarioMaintenanceCreate).
				Return(tt.allowed, tt.authzErr)

			mw := RequireScenario(authorizer, entity.AuthzScenarioMaintenanceCreate)
			nextF := func(c *echo.Context) error {
				return c.NoContent(http.StatusNoContent)
			}
			err := mw(nextF)(c)

			require.NoError(t, err)
			require.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

func TestRequireScenarioUnlessSelf(t *testing.T) {
	t.Parallel()

	self := uuid.New()

	tests := []struct {
		name           string
		pathID         string
		prepareMock    func(*mock_middlewares.MockAuthorizer)
		expectedStatus int
	}{
		{
			name:           "own record passes without the scenario",
			pathID:         self.String(),
			prepareMock:    func(*mock_middlewares.MockAuthorizer) {},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:   "someone else's record needs the scenario",
			pathID: uuid.NewString(),
			prepareMock: func(a *mock_middlewares.MockAuthorizer) {
				a.EXPECT().Allow(gomock.Any(), []entity.Role{entity.RoleGuest}, entity.AuthzScenarioAuthUsersRead).
					Return(false, nil)
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:   "someone else's record with the scenario",
			pathID: uuid.NewString(),
			prepareMock: func(a *mock_middlewares.MockAuthorizer) {
				a.EXPECT().Allow(gomock.Any(), []entity.Role{entity.RoleGuest}, entity.AuthzScenarioAuthUsersRead).
					Return(true, nil)
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:   "a malformed id is not the caller's own",
			pathID: "not-a-uuid",
			prepareMock: func(a *mock_middlewares.MockAuthorizer) {
				a.EXPECT().Allow(gomock.Any(), gomock.Any(), entity.AuthzScenarioAuthUsersRead).Return(false, nil)
			},
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

			authorizer := mock_middlewares.NewMockAuthorizer(gomock.NewController(t))
			tt.prepareMock(authorizer)

			c, rec := echotest.ContextConfig{
				Request: httptest.NewRequestWithContext(ctx, http.MethodGet, "/", http.NoBody),
			}.ToContextRecorder(t)
			c.SetPathValues(echo.PathValues{{Name: "id", Value: tt.pathID}})
			xecho.UserToEchoCtx(c, &entity.User{ID: self, Roles: []entity.Role{entity.RoleGuest}})

			mw := RequireScenarioUnlessSelf(authorizer, entity.AuthzScenarioAuthUsersRead, "id")
			err := mw(func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) })(c)
			require.NoError(t, err)
			require.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}
