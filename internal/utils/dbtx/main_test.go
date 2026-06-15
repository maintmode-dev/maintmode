package dbtx

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/config"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"

	mock_dbtx "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/closer"
)

type CommitRollbacker interface {
	Commit(ctx context.Context, tx *sqlx.Tx) error
	Rollback(ctx context.Context, tx *sqlx.Tx) error
}

var db *sqlx.DB

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	code := m.Run()

	os.Exit(code)
}

type mngrMocks struct {
	dbtx *mock_dbtx.MockCommitRollbacker
}

func initTxManagerWithMock(t *testing.T) (*TxManager, *mngrMocks) {
	t.Helper()

	ctrl := gomock.NewController(t)

	mocks := &mngrMocks{
		dbtx: mock_dbtx.NewMockCommitRollbacker(ctrl),
	}

	mngr := NewTxManager(db)
	// The mock records and asserts commit/rollback calls, but each call must also
	// finalize the REAL transaction WithinTx opened via BeginTxx — otherwise every
	// attempt leaks a pooled connection and a -count>1 run exhausts the pool and
	// hangs on the next BeginTxx. So wrap the mock: invoke it for the assertion,
	// then close the real tx (the mock's return value stays the result).
	mngr.commit = func(ctx context.Context, tx *sqlx.Tx) error {
		mockErr := mocks.dbtx.Commit(ctx, tx)
		_ = tx.Commit()
		return mockErr
	}
	mngr.rollback = func(ctx context.Context, tx *sqlx.Tx) error {
		mockErr := mocks.dbtx.Rollback(ctx, tx)
		_ = tx.Rollback()
		return mockErr
	}
	mngr.sleep = func(time.Duration) {} // don't burn wall-clock on retry backoff

	return mngr, mocks
}
