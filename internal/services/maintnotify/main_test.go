package maintnotify

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/config"
	mock_maintnotify "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/maintnotify"
)

func TestMain(m *testing.M) {
	code := m.Run()

	os.Exit(code)
}

type serviceMocks struct {
	sender       *mock_maintnotify.MockMessageSender
	notifyTarget *mock_maintnotify.MockNotifyTargetsStore
}

func initNotifier(t *testing.T) (*Service, *serviceMocks) {
	t.Helper()
	ctrl := gomock.NewController(t)

	mocks := &serviceMocks{
		sender:       mock_maintnotify.NewMockMessageSender(ctrl),
		notifyTarget: mock_maintnotify.NewMockNotifyTargetsStore(ctrl),
	}

	cfg := &config.AppConfig{
		App: config.App{FrontendURL: "https://maintmode.test"},
	}
	n, err := NewNotifier(cfg,
		mocks.sender,
		mocks.notifyTarget,
	)
	require.NoError(t, err)

	return n, mocks
}
