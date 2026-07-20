package license

import (
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	mock_license "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/license"
)

type serviceMocks struct {
	users       *mock_license.MockUsersStore
	invitations *mock_license.MockInvitationsStore
	audit       *mock_license.MockAuditStore
	store       *mock_license.MockStore
	client      *mock_license.MockHeartbeatClient
}

// newServiceWithMocks builds the service with a long cache TTL so a single
// read populates the cache and later reads in the same test hit it (unless the
// test wants otherwise, it can construct its own).
func newServiceWithMocks(t *testing.T) (*Service, serviceMocks) {
	t.Helper()
	return newServiceWithMocksTTL(t, time.Hour)
}

func newServiceWithMocksTTL(t *testing.T, ttl time.Duration) (*Service, serviceMocks) {
	t.Helper()
	ctrl := gomock.NewController(t)
	m := serviceMocks{
		users:       mock_license.NewMockUsersStore(ctrl),
		invitations: mock_license.NewMockInvitationsStore(ctrl),
		audit:       mock_license.NewMockAuditStore(ctrl),
		store:       mock_license.NewMockStore(ctrl),
		client:      mock_license.NewMockHeartbeatClient(ctrl),
	}
	svc := NewService(m.users, m.invitations, m.audit, m.store, m.client, "v-test", ttl)
	return svc, m
}
