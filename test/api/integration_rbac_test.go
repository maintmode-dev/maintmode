//go:build api

package api

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/xhttp/client"

	"github.com/ruko1202/maintmode/internal/entity"
)

// integrationRequest issues a raw HTTP request to the integration API with a
// role-scoped token. RBAC is exercised over raw HTTP (not the generated client)
// so the requests carry exactly the bytes the table declares.
func integrationRequest(ctx context.Context, t *testing.T, method, path, body string, roles ...entity.Role) int {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, method,
		baseURL("maintmode")+"/api/v1/integrations"+path, bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+mustTestAccessToken(roles...))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.NewClient(client.WithTimeout(5 * time.Second)).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func TestIntegrationAPIRBAC_NonAdminManageForbidden(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	body := `{"kind":"slack","enabled":false,"secrets":{"bot_token":"x"}}`
	for _, role := range []entity.Role{entity.RoleGuest, entity.RoleEditor, entity.RoleReviewer} {
		t.Run(string(role)+" cannot create", func(t *testing.T) {
			status := integrationRequest(ctx, t, http.MethodPost, "", body, role)
			require.Equal(t, http.StatusForbidden, status,
				"a non-admin must be forbidden from managing integrations")
		})
	}
}

func TestIntegrationAPIRBAC_NonAdminReadForbidden(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	status := integrationRequest(ctx, t, http.MethodGet, "", "", entity.RoleGuest)
	require.Equal(t, http.StatusForbidden, status, "a guest must not read integrations")
}

func TestIntegrationAPIRBAC_AdminManageNotForbidden(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	// Admin passes the RBAC gate. The kind is unknown-configured or valid; either
	// way the response must NOT be 401/403 — the admin is authorized to try.
	status := integrationRequest(ctx, t, http.MethodGet, "", "", entity.RoleAdmin)
	require.NotEqual(t, http.StatusForbidden, status)
	require.NotEqual(t, http.StatusUnauthorized, status)
}

// integrationRoute is one wired route with the RBAC scope its scenario middleware
// enforces. This table pins every route->scenario wiring (api_server.go), so a
// route accidentally given the wrong scenario (e.g. read on a mutating endpoint)
// is caught — the gap the two happy-path tests above could not see.
type integrationRoute struct {
	name   string
	method string
	path   string
	body   string
	manage bool // true: integration.manage (admin only); false: integration.read
}

func integrationRoutes() []integrationRoute {
	// enabled:false on purpose: the admin pass-through test actually lands these
	// writes, and the suite runs with the LIVE transport resolver (use_stub off) —
	// an enabled slack row would make every dispatch hit the real Slack API.
	const body = `{"kind":"slack","enabled":false,"config":{},"secrets":{"bot_token":"x"}}`
	return []integrationRoute{
		{"list", http.MethodGet, "", "", false},
		{"get one", http.MethodGet, "/slack", "", false},
		{"create", http.MethodPost, "", body, true},
		{"update", http.MethodPatch, "/slack", body, true},
		{"toggle", http.MethodPost, "/slack/toggle", `{"enabled":false}`, true},
	}
}

// A non-admin is forbidden on every route: read routes reject guest, manage
// routes reject the highest non-admin role (reviewer). This asserts each route
// carries an RBAC scenario at all, and that manage is not accidentally read.
func TestIntegrationAPIRBAC_EveryRouteGated(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	for _, r := range integrationRoutes() {
		t.Run(r.name, func(t *testing.T) {
			// read routes: a guest must be forbidden (only admin has read).
			// manage routes: reviewer (top non-admin) must be forbidden.
			role := entity.RoleGuest
			if r.manage {
				role = entity.RoleReviewer
			}
			status := integrationRequest(ctx, t, r.method, r.path, r.body, role)
			require.Equal(t, http.StatusForbidden, status,
				"%s %s must be gated by its integration scenario", r.method, r.path)
		})
	}
}

// Admin clears the RBAC gate on every route (never 401/403). Confirms the manage
// routes aren't over-restricted and the read routes admit admin too.
func TestIntegrationAPIRBAC_AdminPassesEveryRoute(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	for _, r := range integrationRoutes() {
		t.Run(r.name, func(t *testing.T) {
			status := integrationRequest(ctx, t, r.method, r.path, r.body, entity.RoleAdmin)
			require.NotEqual(t, http.StatusForbidden, status, "admin must pass %s %s", r.method, r.path)
			require.NotEqual(t, http.StatusUnauthorized, status)
		})
	}
}
