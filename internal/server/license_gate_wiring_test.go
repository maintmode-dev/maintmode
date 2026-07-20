package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/server/middlewares"
)

// blockedProvider is a LicenseProvider that always reports a blocked license,
// so the block gate fires on every mutating request routed through it.
type blockedProvider struct{}

func (blockedProvider) License(context.Context) *entity.License {
	return &entity.License{Status: entity.LicenseStatusBlocked}
}

// TestLicenseGateWiring pins the routing invariant that the block gate covers
// the business routes only and never the auth module. It reproduces the group
// topology from BindRouters — auth routes hang off the bare /api/v1 group, the
// business routes hang off a gated child — and asserts, under a blocked
// license, that a mutating auth request still reaches its handler while a
// mutating business request is rejected by the gate before its handler runs.
//
// The regression it guards against is the one where the gate was applied to the
// whole /api/v1 group: that silently blocked logout, /me and provider connect
// for members of a blocked org, who could then sign in but not sign out.
func TestLicenseGateWiring(t *testing.T) {
	t.Parallel()

	const okBody = "handler-reached"

	handler := func(c *echo.Context) error {
		return c.String(http.StatusOK, okBody)
	}

	e := echo.New()
	apiV1 := e.Group("/api/v1")

	// Auth module: registered on the bare group, exactly like authPublicV1Group
	// and authProtectedV1Group. No block gate.
	apiV1.Add(http.MethodPost, "/logout", handler)
	apiV1.Add(http.MethodPatch, "/me", handler)

	// Business routes: registered on the gated child, exactly like apiV1Group.
	gated := apiV1.Group("", middlewares.RequireLicenseNotSuspended(blockedProvider{}))
	gated.Add(http.MethodPost, "/maintenances/create", handler)
	gated.Add(http.MethodGet, "/maintenances/:id", handler)

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{
			name:       "mutating auth request bypasses the gate and reaches the handler",
			method:     http.MethodPost,
			path:       "/api/v1/logout",
			wantStatus: http.StatusOK,
		},
		{
			name:       "mutating profile update bypasses the gate and reaches the handler",
			method:     http.MethodPatch,
			path:       "/api/v1/me",
			wantStatus: http.StatusOK,
		},
		{
			name:       "mutating business request is rejected by the gate",
			method:     http.MethodPost,
			path:       "/api/v1/maintenances/create",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "reading a business resource stays available under a blocked license",
			method:     http.MethodGet,
			path:       "/api/v1/maintenances/some-id",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tt.method, tt.path, http.NoBody)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			require.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}
