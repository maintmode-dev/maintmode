package conflicts

import (
	"testing"

	"go.uber.org/mock/gomock"

	mock_conflicts "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/conflicts"
)

type serviceMocks struct {
	snapshots *mock_conflicts.MockConflictSnapshotsStore
}

// initService builds the service with a mocked snapshot store and no live
// conflicts store. The tests here drive only the snapshot half — the live query
// needs a database and is covered by the integration tests under ./test.
func initService(t *testing.T) (*Service, *serviceMocks) {
	t.Helper()
	ctrl := gomock.NewController(t)

	mocks := &serviceMocks{
		snapshots: mock_conflicts.NewMockConflictSnapshotsStore(ctrl),
	}

	return NewService(nil, mocks.snapshots), mocks
}
