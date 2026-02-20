package dbtx

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/ruko1202/xlog"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

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
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	xlog.ReplaceGlobal(logger)

	conn := testdbconnutils.NewDB()
	closer.Add(conn.Close)
	db = conn

	code := m.Run()

	closer.CloseAll(ctx)

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
	mngr.commit = mocks.dbtx.Commit
	mngr.rollback = mocks.dbtx.Rollback

	return mngr, mocks
}
