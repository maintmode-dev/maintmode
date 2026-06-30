package userpicker

import (
	"os"
	"testing"

	"go.uber.org/mock/gomock"

	mock_userpicker "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/userpicker"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

type serviceMocks struct {
	users *mock_userpicker.MockUserLister
}

func initService(t *testing.T) (*Service, *serviceMocks) {
	t.Helper()
	ctrl := gomock.NewController(t)

	mocks := &serviceMocks{
		users: mock_userpicker.NewMockUserLister(ctrl),
	}

	return NewService(mocks.users), mocks
}
