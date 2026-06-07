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
)

// TestNotifyTransportsAPI_Catalog verifies GET /notifications/transports returns
// the slack+telegram MVP catalog with the UX copy the channel-create form needs,
// and that every advertised id is a transport a channel can be created on.
func TestNotifyTransportsAPI_Catalog(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient() // admin

	resp, err := apiClient.GetApiV1NotificationsTransportsWithResponse(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	transports := lo.FromPtr(resp.JSON200.Transports)
	ids := lo.Map(transports, func(tr maintmodeclient.ApimodelsTransport, _ int) string {
		return lo.FromPtr(tr.Id)
	})
	require.Equal(t, []string{
		string(entity.NotifyTransportSlack),
		string(entity.NotifyTransportTelegram),
	}, ids)

	// Every entry carries the full UX copy; ids must round-trip POST /channels.
	for _, tr := range transports {
		require.NotEmpty(t, lo.FromPtr(tr.Title))
		require.True(t, entity.NotifyTransport(lo.FromPtr(tr.Id)).IsValid(),
			"catalog transport %q must be accepted by NotifyTransport.IsValid", lo.FromPtr(tr.Id))
	}
}

// TestNotifyTransportsAPI_GuestAllowed verifies the catalog is a reference list
// available to any authenticated role: a read-only guest gets 200, not 403.
func TestNotifyTransportsAPI_GuestAllowed(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	guest := setupMaintmodeTestClientWithRoles(entity.RoleGuest)

	resp, err := guest.GetApiV1NotificationsTransportsWithResponse(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), "guest must read the catalog: %s", resp.Body)
}
