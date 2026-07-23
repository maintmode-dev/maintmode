package license

import (
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/config"
	mock_license "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/license"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

// db backs the DB-backed guard tests: EnsureSeatAvailable takes a real
// transaction-scoped advisory lock, so its paths that reach the lock need a live
// tx (the mock stores still supply the role lists). Paths that return before the
// lock (nil license / nil cap / Noop) need no DB.
var db *sqlx.DB

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	os.Exit(m.Run())
}

// txManager builds a transaction manager over the shared test DB so guard tests
// can invoke EnsureSeatAvailable inside a real tx.
func txManager() *dbtx.TxManager { return dbtx.NewTxManager(db) }

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
