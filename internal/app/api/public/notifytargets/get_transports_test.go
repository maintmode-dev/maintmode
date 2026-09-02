package apinotifications

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/notifytargets/models"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	mock_apinotifications "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/api/notifytargets"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

// transportsImpl builds an Implementation whose transport source yields the
// given resolve outcome per transport — the handler resolves each catalog
// transport exactly once per request.
func transportsImpl(t *testing.T, outcomes map[entity.NotifyTransport]error) *Implementation {
	t.Helper()

	transports := mock_apinotifications.NewMocktransportSource(gomock.NewController(t))
	for transport, resolveErr := range outcomes {
		transports.EXPECT().Get(gomock.Any(), transport).Return(nil, resolveErr)
	}
	return &Implementation{transports: transports}
}

// TestGetTransports covers the transports catalog handler: the static
// slack/telegram catalog enriched with per-transport transport_status derived
// from the delivery resolver.
func TestGetTransports(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		// slack resolves, telegram exists but is disabled; the classification of
		// each catalog transport lands in the response.
		impl := transportsImpl(t, map[entity.NotifyTransport]error{
			entity.NotifyTransportSlack:    nil,
			entity.NotifyTransportTelegram: apperr.ErrIntegrationDisabled,
		})

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)

		err := impl.GetTransports(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.TransportsResponse](t, rec.Body)

		// Catalog ids must be the slack/telegram MVP set, every id must be a
		// transport a channel can actually be created on, and each entry must
		// carry the status derived from the resolve outcome above.
		wantStatuses := map[string]apimodels.TransportStatus{
			string(entity.NotifyTransportSlack):    apimodels.TransportStatusOK,
			string(entity.NotifyTransportTelegram): apimodels.TransportStatusDisabled,
		}
		ids := make([]string, 0, len(resp.Transports))
		for _, tr := range resp.Transports {
			ids = append(ids, tr.ID)
			require.True(t, entity.NotifyTransport(tr.ID).IsValid(),
				"catalog transport %q must be accepted by NotifyTransport.IsValid", tr.ID)
			require.Equal(t, wantStatuses[tr.ID], tr.TransportStatus,
				"transport %q status", tr.ID)
		}
		require.Equal(t, []string{
			string(entity.NotifyTransportSlack),
			string(entity.NotifyTransportTelegram),
		}, ids)

		// The shared static catalog must stay untouched by the per-request
		// status projection.
		for _, tr := range apimodels.SupportedTransports {
			require.Empty(t, tr.TransportStatus,
				"SupportedTransports must not be mutated by the handler")
		}
	})

	t.Run("unreadable and not_configured are distinguished", func(t *testing.T) {
		t.Parallel()

		impl := transportsImpl(t, map[entity.NotifyTransport]error{
			entity.NotifyTransportSlack: fmt.Errorf("%w: unwrap dek", apperr.ErrIntegrationUnreadable),
			entity.NotifyTransportTelegram: fmt.Errorf("%w: %q",
				apperr.ErrIntegrationNotConfigured, entity.NotifyTransportTelegram),
		})
		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)

		require.NoError(t, impl.GetTransports(c))
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.TransportsResponse](t, rec.Body)
		got := map[string]apimodels.TransportStatus{}
		for _, tr := range resp.Transports {
			got[tr.ID] = tr.TransportStatus
		}
		require.Equal(t, apimodels.TransportStatusUnreadable, got[string(entity.NotifyTransportSlack)])
		require.Equal(t, apimodels.TransportStatusNotConfigured, got[string(entity.NotifyTransportTelegram)])
	})

	// transport_status is a mandatory read-model field: an unclassifiable
	// resolve failure (storage outage, not an integration state) must fail the
	// request loudly (500), never degrade to a made-up status. All five
	// handlers funnel through the same transportStatuses helper, so this pins
	// the shared error path.
	t.Run("unclassified resolve failure returns 500", func(t *testing.T) {
		t.Parallel()

		impl := transportsImpl(t, map[entity.NotifyTransport]error{
			entity.NotifyTransportSlack: errors.New("pq: connection refused"),
		})
		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)

		err := impl.GetTransports(c)
		require.NoError(t, err, "ToAPIError writes the response itself")
		require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
		require.NotContains(t, rec.Body.String(), `"transports":`,
			"failure must return the error envelope, not a partial catalog")
	})
}
