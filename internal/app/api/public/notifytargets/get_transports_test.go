package apinotifications

import (
	"errors"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/notifytargets/models"
	"github.com/ruko1202/maintmode/internal/entity"
	mock_apinotifications "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/api/notifytargets"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

// transportsImpl builds an Implementation whose registry port returns the
// given masked listing (or error) once — the handler must issue exactly one
// List call per request.
func transportsImpl(t *testing.T, list []*entity.MaskedIntegration, err error) *Implementation {
	t.Helper()

	integrations := mock_apinotifications.NewMockintegrationSource(gomock.NewController(t))
	integrations.EXPECT().List(gomock.Any()).Return(list, err)
	return &Implementation{integrations: integrations}
}

// TestGetTransports covers the transports catalog handler: the static
// slack/telegram catalog enriched with per-transport transport_status derived
// from the integration registry (RUK-198).
func TestGetTransports(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		// slack enabled, telegram present but disabled; the registry knows
		// nothing about any other transport.
		impl := transportsImpl(t, []*entity.MaskedIntegration{
			{Kind: string(entity.NotifyTransportSlack), Enabled: true},
			{Kind: string(entity.NotifyTransportTelegram), Enabled: false},
		}, nil)

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)

		err := impl.GetTransports(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.TransportsResponse](t, rec.Body)

		// Catalog ids must be the slack/telegram MVP set, every id must be a
		// transport a channel can actually be created on, and each entry must
		// carry the status derived from the registry state above.
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

	t.Run("empty registry means not_configured", func(t *testing.T) {
		t.Parallel()

		impl := transportsImpl(t, nil, nil)
		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)

		err := impl.GetTransports(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.TransportsResponse](t, rec.Body)
		for _, tr := range resp.Transports {
			require.Equal(t, apimodels.TransportStatusNotConfigured, tr.TransportStatus,
				"transport %q with no registry row", tr.ID)
		}
	})

	// transport_status is a mandatory read-model field: a registry read failure
	// must fail the request loudly (500), never degrade to a made-up status.
	// All five handlers funnel through the same integrationIndex helper, so
	// this pins the shared error path.
	t.Run("registry read failure returns 500", func(t *testing.T) {
		t.Parallel()

		impl := transportsImpl(t, nil, errors.New("registry unavailable"))
		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)

		err := impl.GetTransports(c)
		require.NoError(t, err, "ToAPIError writes the response itself")
		require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
		require.NotContains(t, rec.Body.String(), `"transports":`,
			"failure must return the error envelope, not a partial catalog")
	})
}
