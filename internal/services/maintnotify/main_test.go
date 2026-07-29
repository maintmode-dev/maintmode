package maintnotify

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/config"
	mock_maintnotify "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/maintnotify"
)

// testMetricReader is installed as the global meter provider before the metrics
// package resolves its instruments, so counter assertions in this package
// observe real values instead of otel's no-op meter — under which every count
// reads zero and the assertions would pass whatever the code did.
var testMetricReader sdkmetric.Reader

func TestMain(m *testing.M) {
	testMetricReader = sdkmetric.NewManualReader()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(testMetricReader)))

	code := m.Run()

	os.Exit(code)
}

type serviceMocks struct {
	sender          *mock_maintnotify.MockMessageSender
	notifyTarget    *mock_maintnotify.MockNotifyTargetsStore
	ownerResolver   *mock_maintnotify.MockOwnerResolver
	mentions        *mock_maintnotify.MockMentionsReader
	mentionResolver *mock_maintnotify.MockMentionResolver
}

func initNotifier(t *testing.T) (*Service, *serviceMocks) {
	t.Helper()
	ctrl := gomock.NewController(t)

	mocks := &serviceMocks{
		sender:          mock_maintnotify.NewMockMessageSender(ctrl),
		notifyTarget:    mock_maintnotify.NewMockNotifyTargetsStore(ctrl),
		ownerResolver:   mock_maintnotify.NewMockOwnerResolver(ctrl),
		mentions:        mock_maintnotify.NewMockMentionsReader(ctrl),
		mentionResolver: mock_maintnotify.NewMockMentionResolver(ctrl),
	}

	cfg := &config.AppConfig{
		App: config.App{FrontendURL: "https://maintmode.test"},
	}
	n, err := NewNotifier(cfg,
		mocks.sender,
		mocks.notifyTarget,
		mocks.ownerResolver,
		mocks.mentions,
		mocks.mentionResolver,
	)
	require.NoError(t, err)

	return n, mocks
}

// expectNoMentions is the default for tests that predate the mentions feature:
// the maintenance has none, so the load returns empty and no resolve follows.
// Spelled out rather than defaulted inside initNotifier so each test still
// declares what it expects of the mentions path.
func expectNoMentions(mocks *serviceMocks) {
	mocks.mentions.EXPECT().
		GetMaintMentions(gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()
}
