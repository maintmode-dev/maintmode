package usersummary

import (
	"os"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	mock_usersummary "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/usersummary"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

type serviceMocks struct {
	users *mock_usersummary.MockUserLister
}

func initService(t *testing.T) (*Service, *serviceMocks) {
	t.Helper()
	ctrl := gomock.NewController(t)

	mocks := &serviceMocks{
		users: mock_usersummary.NewMockUserLister(ctrl),
	}

	return NewService(mocks.users), mocks
}

// initServiceWithTTL is like initService but overrides the cache TTL, so the
// expiry test can use a short real TTL instead of waiting a minute.
func initServiceWithTTL(t *testing.T, ttl time.Duration) (*Service, *serviceMocks) {
	t.Helper()
	ctrl := gomock.NewController(t)

	mocks := &serviceMocks{
		users: mock_usersummary.NewMockUserLister(ctrl),
	}

	return newServiceWithTTL(mocks.users, ttl), mocks
}
