package apinotifications

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
	mock_apinotifications "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/api/notifytargets"
	mock_usersummary "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/usersummary"
	"github.com/ruko1202/maintmode/internal/services/usersummary"
)

// TestGetChannels_QueryToCmd pins how query params become the command the
// service receives.
//
// Handler level rather than API level on purpose. The name normalisation cannot
// be observed from outside: a `name=%20%20` request never reaches the handler
// with spaces intact — measured against the running stack, the layers above
// hand it over already empty, so a black-box test passes whether or not the
// handler trims. Asserting on the command is the only place the rule is
// actually visible.
func TestGetChannels_QueryToCmd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query url.Values
		want  entity.ListChannelsCmd
	}{
		{
			name:  "no params take the helper's defaults",
			query: url.Values{},
			want:  entity.ListChannelsCmd{Name: "", Limit: 50, Offset: 0, IncludeArchived: false},
		},
		{
			name:  "a name is passed through and paging defaults apply",
			query: url.Values{"name": {"alerts"}},
			want:  entity.ListChannelsCmd{Name: "alerts", Limit: 50, Offset: 0},
		},
		{
			// The guard that keeps a blank search from becoming LIKE '%   %',
			// which matches only names containing three spaces.
			name:  "a whitespace-only name is trimmed away entirely",
			query: url.Values{"name": {"   "}},
			want:  entity.ListChannelsCmd{Name: "", Limit: 50, Offset: 0},
		},
		{
			name:  "surrounding whitespace is trimmed off a real query",
			query: url.Values{"name": {"  alerts  "}},
			want:  entity.ListChannelsCmd{Name: "alerts", Limit: 50, Offset: 0},
		},
		{
			name:  "paging is taken as given inside the allowed range",
			query: url.Values{"limit": {"10"}, "offset": {"20"}},
			want:  entity.ListChannelsCmd{Limit: 10, Offset: 20},
		},
		{
			// Unparseable: the helper reports an error the handler drops, so a
			// read-only listing answers with a page instead of a 400.
			name:  "an unparseable limit falls back to the default",
			query: url.Values{"limit": {"abc"}},
			want:  entity.ListChannelsCmd{Limit: 50, Offset: 0},
		},
		{
			name:  "include_archived widens the scope",
			query: url.Values{"include_archived": {"true"}},
			want:  entity.ListChannelsCmd{Limit: 50, IncludeArchived: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			impl, channelSvc := listChannelsMocks(t)

			var got *entity.ListChannelsCmd
			channelSvc.EXPECT().AvailableChannels(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, cmd *entity.ListChannelsCmd) (*entity.ListChannelsResult, error) {
					got = cmd
					return &entity.ListChannelsResult{Channels: nil, Total: 0}, nil
				})

			c, rec := echotest.ContextConfig{QueryValues: tt.query}.ToContextRecorder(t)

			require.NoError(t, impl.GetChannels(c))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.NotNil(t, got, "the handler must reach the service")
			require.Equal(t, tt.want, *got)
		})
	}
}

// listChannelsMocks wires an Implementation for the read path: a channel
// service to record the command, and a transport source that is never consulted
// because every case returns an empty page.
func listChannelsMocks(t *testing.T) (*Implementation, *mock_apinotifications.MockchannelService) {
	t.Helper()
	ctrl := gomock.NewController(t)

	channelSvc := mock_apinotifications.NewMockchannelService(ctrl)
	transports := mock_apinotifications.NewMocktransportSource(ctrl)
	userLister := mock_usersummary.NewMockUserLister(ctrl)

	return New(channelSvc, usersummary.NewService(userLister), transports), channelSvc
}
