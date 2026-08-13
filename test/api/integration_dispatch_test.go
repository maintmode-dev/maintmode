//go:build api

package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/xhttp/client"

	"github.com/ruko1202/maintmode/internal/entity"
	maintmodeclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/maintmode"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// This file is the API-level smoke for delivery on DB-backed settings
// (RUK-196, SPEC §11): the test binary runs with notify_transport.use_stub
// OFF, so a dispatch resolves its transport through the live path — registry
// row in integration_settings -> decrypt -> builder — instead of the stub. No
// integration is ever enabled here, so the resolve ends in the best-effort
// drop; that a delivery actually REACHES a transport when enabled is pinned at
// package level (services/transportresolver e2e), where the client can be
// faked without real Slack credentials.

// adminIntegrationRequest issues a raw admin request to the integration API and
// returns the status code plus response body (the generated-client shape is not
// needed; the masking assertion wants the raw bytes anyway).
func adminIntegrationRequest(ctx context.Context, t *testing.T, method, path, body string) (status int, respBody string) {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, method,
		baseURL("maintmode")+"/api/v1/integrations"+path, bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+mustTestAccessToken(entity.RoleAdmin))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.NewClient(client.WithTimeout(5 * time.Second)).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(raw)
}

// ensureDisabledSlackIntegration drives the slack integration to a known state
// (disabled, with the given bot token) through the public API only. The row may
// already exist — the suite runs with -count=2 on a shared DB and UNIQUE(kind)
// forbids a per-run kind — so create falls back to update on conflict.
func ensureDisabledSlackIntegration(ctx context.Context, t *testing.T, botToken string) {
	t.Helper()

	body := `{"kind":"slack","enabled":false,"config":{},"secrets":{"bot_token":"` + botToken + `"}}`
	status, respBody := adminIntegrationRequest(ctx, t, http.MethodPost, "", body)
	if status == http.StatusConflict {
		status, respBody = adminIntegrationRequest(ctx, t, http.MethodPatch, "/slack",
			`{"enabled":false,"secrets":{"bot_token":"`+botToken+`"}}`)
	}
	require.Equal(t, http.StatusOK, status, "ensure disabled slack integration: %s", respBody)
}

// TestIntegrationDispatch_DisabledSlackDropsDelivery pins the DB-settings
// delivery pipeline end-to-end through the running binary: an admin configures
// a DISABLED slack integration via the API, a maintenance with a slack notify
// target goes planned -> in_progress (which dispatches the maint.started
// notification), and the whole lifecycle stays healthy — the async delivery
// resolves the integration from the DB, sees enabled=false, and drops
// best-effort instead of erroring or retry-storming.
func TestIntegrationDispatch_DisabledSlackDropsDelivery(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient()

	// Per-run token value: the masking assertion below must be able to tell THIS
	// run's plaintext from any leftover of a previous one.
	botToken := "xoxb-smoke-" + xuuid.NewString()
	ensureDisabledSlackIntegration(ctx, t, botToken)

	// Read side never surfaces the secret: only the is-set flag comes back.
	status, body := adminIntegrationRequest(ctx, t, http.MethodGet, "/slack", "")
	require.Equal(t, http.StatusOK, status, "get slack integration: %s", body)
	require.Contains(t, body, `"secrets_set"`, "read must expose the is-set view")
	require.NotContains(t, body, botToken, "read must never surface the plaintext secret")

	// Maintenance with a slack-channel notify target; starting it fires the
	// maint.started notification at that target.
	channelID := ensureNotifyChannel(ctx, t, apiClient)
	maintenanceID := createMaintenanceWithChannel(ctx, t, apiClient, channelID)
	approveMaintenance(ctx, t, apiClient, maintenanceID)

	startResp, err := apiClient.PostApiV1MaintenancesIdStartWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err, "Failed to start maintenance")
	require.Equal(t, http.StatusNoContent, startResp.StatusCode(),
		"start must succeed while its notification drops best-effort: %s", startResp.Body)

	// The dispatch is async and the drop is silent by design — the API-visible
	// contract is that the lifecycle is unaffected. The maintenance must be
	// in_progress, not wedged by the undeliverable notification.
	getResp, err := apiClient.GetApiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, getResp.StatusCode())
	require.NotNil(t, getResp.JSON200)
	require.Equal(t, string(maintmodeclient.MaintenanceStatusInProgress), lo.FromPtr(getResp.JSON200.Status))
}
