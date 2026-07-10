package apinotifications

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/notifytargets/models"
	"github.com/ruko1202/maintmode/internal/entity"
	mock_apinotifications "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/api/notifytargets"
	mock_usersummary "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/usersummary"
	"github.com/ruko1202/maintmode/internal/services/usersummary"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

// registryFailureMocks wires an Implementation whose registry port always
// fails, backed by gomock ports. Write expectations are intentionally NOT set
// on the channel service: gomock fails the test on any unexpected call, which
// is exactly the index-before-write assertion — a registry failure must
// short-circuit before the service write is reached.
func registryFailureMocks(t *testing.T) (*Implementation, *mock_apinotifications.MockchannelService) {
	t.Helper()
	ctrl := gomock.NewController(t)

	channelSvc := mock_apinotifications.NewMockchannelService(ctrl)

	integrations := mock_apinotifications.NewMockintegrationSource(ctrl)
	integrations.EXPECT().List(gomock.Any()).Return(nil, errors.New("registry unavailable"))

	// The resolver is exercised through its documented degrade-never-fail
	// contract; the canned channel carries no author ids, so the lister is
	// never consulted (no expectations).
	userLister := mock_usersummary.NewMockUserLister(ctrl)

	return New(channelSvc, usersummary.NewService(userLister), integrations), channelSvc
}

func testChannel() *entity.NotifyChannel {
	return &entity.NotifyChannel{
		ID:                 uuid.New(),
		Transport:          entity.NotifyTransportSlack,
		TransportChannelID: "C-unit",
		Name:               "unit channel",
		CreatedAt:          time.Now(),
	}
}

func testUser() *entity.User {
	return &entity.User{ID: uuid.New(), Name: "unit tester", Email: "unit@test.local"}
}

// TestChannelHandlers_RegistryFailure500 pins, per handler, that an
// integrationIndex failure is propagated as the standard 500 envelope:
// transport_status is a mandatory read-model field, so no handler may swallow
// the error or degrade to a made-up status. For create/update it additionally
// pins that the failure short-circuits BEFORE the write (no CreateChannel /
// UpdateChannel expectation is registered, so reaching the write fails the
// test) — a committed mutation reported as 500 would turn a client retry into
// a spurious 409.
func TestChannelHandlers_RegistryFailure500(t *testing.T) {
	t.Parallel()

	t.Run("GetChannel", func(t *testing.T) {
		t.Parallel()

		impl, channelSvc := registryFailureMocks(t)
		channelSvc.EXPECT().GetChannel(gomock.Any(), gomock.Any()).Return(testChannel(), nil)

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{{Name: "id", Value: uuid.NewString()}})

		require.NoError(t, impl.GetChannel(c))
		require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	})

	t.Run("GetChannels", func(t *testing.T) {
		t.Parallel()

		impl, channelSvc := registryFailureMocks(t)
		channelSvc.EXPECT().AvailableChannels(gomock.Any(), false).
			Return([]*entity.NotifyChannel{testChannel()}, nil)

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)

		require.NoError(t, impl.GetChannels(c))
		require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	})

	t.Run("CreateChannel fails before the write", func(t *testing.T) {
		t.Parallel()

		impl, _ := registryFailureMocks(t)

		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, &apimodels.CreateChannelRequest{
				Transport:          string(entity.NotifyTransportSlack),
				TransportChannelID: "C-unit",
				Name:               "unit channel",
			}),
		}.ToContextRecorder(t)
		xecho.UserToEchoCtx(c, testUser())

		require.NoError(t, impl.CreateChannel(c))
		require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	})

	t.Run("UpdateChannel fails before the write", func(t *testing.T) {
		t.Parallel()

		impl, _ := registryFailureMocks(t)

		newName := "renamed"
		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, &apimodels.UpdateChannelRequest{
				Name: &newName,
			}),
		}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{{Name: "id", Value: uuid.NewString()}})
		xecho.UserToEchoCtx(c, testUser())

		require.NoError(t, impl.UpdateChannel(c))
		require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	})
}
