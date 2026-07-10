//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	maintmodeclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/maintmode"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// ensureTelegramIntegration drives the telegram integration row to the wanted
// enabled state through the public API only. The row may already exist — the
// suite runs with -count=2 on a shared DB and UNIQUE(kind) forbids a per-run
// kind — so create falls back to toggle on conflict.
//
// CAUTION: the suite runs the LIVE resolver (use_stub off). While the row is
// enabled, any concurrently pending telegram delivery would build a real
// transport around the bogus token and attempt an outbound Telegram call
// (failing fast, but network egress). Keep the enabled window short and last
// in the test, and always restore disabled via t.Cleanup so a passing or
// failing run leaves the shared DB safe. A killed process (cleanup never runs)
// can still leak enabled=true — the next run's initial ensure(false) heals it.
func ensureTelegramIntegration(ctx context.Context, t *testing.T, enabled bool) {
	t.Helper()

	enabledJSON := "false"
	if enabled {
		enabledJSON = "true"
	}
	body := `{"kind":"telegram","enabled":` + enabledJSON + `,"config":{},"secrets":{"bot_token":"api-test-bogus"}}`
	status, respBody := adminIntegrationRequest(ctx, t, http.MethodPost, "", body)
	if status == http.StatusConflict {
		status, respBody = adminIntegrationRequest(ctx, t, http.MethodPost, "/telegram/toggle",
			`{"enabled":`+enabledJSON+`}`)
	}
	require.Equal(t, http.StatusOK, status, "ensure telegram integration enabled=%s: %s", enabledJSON, respBody)
}

// TestNotifyTransportStatusAPI pins the RUK-198 variant-В contract end-to-end:
// the transport↔kind coupling stays weak (a channel is created on a disabled
// integration without complaint), but every channel read model and the
// transports catalog surface transport_status so the FE can flag silent
// non-delivery. The not_configured branch is pinned at handler/model level
// (no DELETE /integrations route exists to remove a kind row here).
func TestNotifyTransportStatusAPI(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient() // admin

	// Leave the shared-DB row in the safe state for the rest of the suite.
	t.Cleanup(func() { ensureTelegramIntegration(ctx, t, false) })

	ensureTelegramIntegration(ctx, t, false)

	// Weak coupling: create succeeds even though the integration is disabled,
	// and the response already carries the disabled signal.
	createResp, err := apiClient.PostApiV1NotificationsChannelsWithResponse(ctx,
		maintmodeclient.PostApiV1NotificationsChannelsJSONRequestBody{
			Transport:          lo.ToPtr(string(entity.NotifyTransportTelegram)),
			TransportChannelId: lo.ToPtr("status-" + xuuid.NewString()),
			Name:               lo.ToPtr("Transport status channel"),
			Description:        lo.ToPtr("created by API test"),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode(), "unexpected status: %s", createResp.Body)
	require.NotNil(t, createResp.JSON201)
	require.Equal(t, maintmodeclient.TransportStatusDisabled,
		lo.FromPtr(createResp.JSON201.TransportStatus), "create response on disabled integration")

	channelID := lo.FromPtr(createResp.JSON201.Id)

	getResp, err := apiClient.GetApiV1NotificationsChannelsIdWithResponse(ctx, channelID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, getResp.StatusCode(), "unexpected status: %s", getResp.Body)
	require.Equal(t, maintmodeclient.TransportStatusDisabled,
		lo.FromPtr(getResp.JSON200.TransportStatus), "GET channel on disabled integration")

	updateResp, err := apiClient.PatchApiV1NotificationsChannelsIdWithResponse(ctx, channelID,
		maintmodeclient.PatchApiV1NotificationsChannelsIdJSONRequestBody{
			Description: lo.ToPtr("updated by API test"),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, updateResp.StatusCode(), "unexpected status: %s", updateResp.Body)
	require.Equal(t, maintmodeclient.TransportStatusDisabled,
		lo.FromPtr(updateResp.JSON200.TransportStatus), "PATCH response on disabled integration")

	listResp, err := apiClient.GetApiV1NotificationsChannelsWithResponse(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, listResp.StatusCode())
	listed, found := lo.Find(lo.FromPtr(listResp.JSON200.Channels),
		func(ch maintmodeclient.ApimodelsChannel) bool { return lo.FromPtr(ch.Id) == channelID })
	require.True(t, found, "created channel must be listed")
	require.Equal(t, maintmodeclient.TransportStatusDisabled,
		lo.FromPtr(listed.TransportStatus), "LIST channels on disabled integration")

	transportStatus := func(t *testing.T) maintmodeclient.ApimodelsTransportStatus {
		t.Helper()
		resp, err := apiClient.GetApiV1NotificationsTransportsWithResponse(ctx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		tr, found := lo.Find(lo.FromPtr(resp.JSON200.Transports),
			func(tr maintmodeclient.ApimodelsTransport) bool {
				return lo.FromPtr(tr.Id) == string(entity.NotifyTransportTelegram)
			})
		require.True(t, found, "telegram must be in the transports catalog")
		return lo.FromPtr(tr.TransportStatus)
	}
	require.Equal(t, maintmodeclient.TransportStatusDisabled, transportStatus(t),
		"GET /transports on disabled integration")

	// Enabling the integration flips the signal to ok on both surfaces without
	// touching the channel.
	ensureTelegramIntegration(ctx, t, true)

	getResp, err = apiClient.GetApiV1NotificationsChannelsIdWithResponse(ctx, channelID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, getResp.StatusCode())
	require.Equal(t, maintmodeclient.TransportStatusOK,
		lo.FromPtr(getResp.JSON200.TransportStatus), "GET channel on enabled integration")

	require.Equal(t, maintmodeclient.TransportStatusOK, transportStatus(t),
		"GET /transports on enabled integration")

	// Create inside the enabled window: the create response itself must carry ok.
	okCreateResp, err := apiClient.PostApiV1NotificationsChannelsWithResponse(ctx,
		maintmodeclient.PostApiV1NotificationsChannelsJSONRequestBody{
			Transport:          lo.ToPtr(string(entity.NotifyTransportTelegram)),
			TransportChannelId: lo.ToPtr("status-ok-" + xuuid.NewString()),
			Name:               lo.ToPtr("Transport status ok channel"),
			Description:        lo.ToPtr("created by API test"),
		},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, okCreateResp.StatusCode(), "unexpected status: %s", okCreateResp.Body)
	require.Equal(t, maintmodeclient.TransportStatusOK,
		lo.FromPtr(okCreateResp.JSON201.TransportStatus), "create response on enabled integration")
}
