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

	"github.com/ruko1202/maintmode/internal/entity"
	mock_middlewares "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/server/middlewares"
)

func TestRequireLicenseNotSuspended(t *testing.T) {
	t.Parallel()

	blocked := &entity.License{Status: entity.LicenseStatusBlocked}
	active := &entity.License{Status: entity.LicenseStatusActive}

	tests := []struct {
		name           string
		method         string
		license        *entity.License
		expectMutation bool // whether the license is consulted at all
		expectedStatus int
	}{
		{
			name:           "blocked license blocks a mutating request",
			method:         http.MethodPost,
			license:        blocked,
			expectMutation: true,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "blocked license leaves a read request available",
			method:         http.MethodGet,
			license:        blocked,
			expectMutation: false, // read is short-circuited before the license is read
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "active passes a mutating request",
			method:         http.MethodPost,
			license:        active,
			expectMutation: true,
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "no license known yet passes a mutating request",
			method:         http.MethodPost,
			license:        nil,
			expectMutation: true,
			expectedStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

			c, rec := echotest.ContextConfig{
				Request: httptest.NewRequestWithContext(ctx, tt.method, "/", http.NoBody),
			}.ToContextRecorder(t)

			provider := mock_middlewares.NewMockLicenseProvider(gomock.NewController(t))
			if tt.expectMutation {
				provider.EXPECT().License(gomock.Any()).Return(tt.license)
			}

			mw := RequireLicenseNotSuspended(provider)
			err := mw(func(c *echo.Context) error {
				return c.NoContent(http.StatusNoContent)
			})(c)
			require.NoError(t, err)
			require.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}
