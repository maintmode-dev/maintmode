package apinotifications

import (
	"net/http"
	"testing"

	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/notifytargets/models"
	"github.com/ruko1202/maintmode/internal/entity"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

// TestGetTransports covers the static transports catalog handler. It needs no
// service, so the implementation is constructed empty.
func TestGetTransports(t *testing.T) {
	t.Parallel()

	impl := &Implementation{}

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)

		err := impl.GetTransports(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.TransportsResponse](t, rec.Body)
		require.Equal(t, apimodels.SupportedTransports, resp.Transports)

		// Catalog ids must be the slack/telegram MVP set, and every id must be
		// a transport a channel can actually be created on.
		ids := make([]string, 0, len(resp.Transports))
		for _, tr := range resp.Transports {
			ids = append(ids, tr.ID)
			require.True(t, entity.NotifyTransport(tr.ID).IsValid(),
				"catalog transport %q must be accepted by NotifyTransport.IsValid", tr.ID)
		}
		require.Equal(t, []string{
			string(entity.NotifyTransportSlack),
			string(entity.NotifyTransportTelegram),
		}, ids)
	})
}
