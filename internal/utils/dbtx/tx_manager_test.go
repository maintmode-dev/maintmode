package dbtx

import (
	"context"
	"fmt"
	"testing"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"
)

func TestTxManager(t *testing.T) {
	l := zaptest.NewLogger(t)
	ctx := xlog.ContextWithLogger(context.Background(), l)

	t.Run("ok", func(t *testing.T) {
		mngr, mocks := initTxManagerWithMock(t)

		mocks.dbtx.EXPECT().
			Commit(gomock.Any(), gomock.Any()).
			Return(nil)

		err := mngr.WithinTx(ctx, func(_ context.Context) error {
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("rollback", func(t *testing.T) {
		mngr, mocks := initTxManagerWithMock(t)

		mocks.dbtx.EXPECT().
			Rollback(gomock.Any(), gomock.Any()).
			Return(nil)

		err := mngr.WithinTx(ctx, func(_ context.Context) error {
			return fmt.Errorf("some err")
		})
		require.EqualError(t, err, "some err")
	})

	t.Run("panic", func(t *testing.T) {
		mngr, mocks := initTxManagerWithMock(t)

		mocks.dbtx.EXPECT().
			Rollback(gomock.Any(), gomock.Any()).
			Return(nil)

		err := mngr.WithinTx(ctx, func(_ context.Context) error {
			panic("some panic")
		})
		require.EqualError(t, err, "panic recovery: some panic")
	})
}
