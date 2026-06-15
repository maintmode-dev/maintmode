package dbtx

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"
)

func TestTxManager(t *testing.T) {
	l := zaptest.NewLogger(t)
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(l))

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

func TestWithinSerializableTx(t *testing.T) {
	l := zaptest.NewLogger(t)
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(l))

	serializationErr := &pq.Error{Code: ErrPGSerializationFailure}

	t.Run("retries on serialization failure then succeeds", func(t *testing.T) {
		mngr, mocks := initTxManagerWithMock(t)

		// First two attempts conflict and roll back; the third commits.
		mocks.dbtx.EXPECT().Rollback(gomock.Any(), gomock.Any()).Return(nil).Times(2)
		mocks.dbtx.EXPECT().Commit(gomock.Any(), gomock.Any()).Return(nil).Times(1)

		attempts := 0
		err := mngr.WithinSerializableTx(ctx, func(_ context.Context) error {
			attempts++
			if attempts < 3 {
				return serializationErr
			}
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, 3, attempts)
	})

	t.Run("gives up after max attempts", func(t *testing.T) {
		mngr, mocks := initTxManagerWithMock(t)

		mocks.dbtx.EXPECT().Rollback(gomock.Any(), gomock.Any()).Return(nil).Times(serializableMaxAttempts)

		attempts := 0
		err := mngr.WithinSerializableTx(ctx, func(_ context.Context) error {
			attempts++
			return serializationErr
		})
		require.ErrorIs(t, err, serializationErr)
		require.Equal(t, serializableMaxAttempts, attempts)
	})

	t.Run("non-serialization error is not retried", func(t *testing.T) {
		mngr, mocks := initTxManagerWithMock(t)

		mocks.dbtx.EXPECT().Rollback(gomock.Any(), gomock.Any()).Return(nil).Times(1)

		attempts := 0
		err := mngr.WithinSerializableTx(ctx, func(_ context.Context) error {
			attempts++
			return fmt.Errorf("some err")
		})
		require.EqualError(t, err, "some err")
		require.Equal(t, 1, attempts)
	})

	t.Run("backs off between retries", func(t *testing.T) {
		mngr, mocks := initTxManagerWithMock(t)
		mocks.dbtx.EXPECT().Rollback(gomock.Any(), gomock.Any()).Return(nil).Times(serializableMaxAttempts)

		var sleeps []time.Duration
		mngr.sleep = func(d time.Duration) { sleeps = append(sleeps, d) }

		err := mngr.WithinSerializableTx(ctx, func(_ context.Context) error {
			return serializationErr
		})
		require.ErrorIs(t, err, serializationErr)
		// One backoff between each attempt, none after the final one.
		require.Len(t, sleeps, serializableMaxAttempts-1)
		for _, d := range sleeps {
			require.GreaterOrEqual(t, d, time.Duration(0))
			require.LessOrEqual(t, d, serializableRetryMaxDelay)
		}
	})
}

func TestSerializableRetryDelay(t *testing.T) {
	// Full-jitter backoff: each delay stays within [0, capped-exponential].
	for attempt := 1; attempt <= 10; attempt++ {
		upper := min(serializableRetryBaseDelay<<(attempt-1), serializableRetryMaxDelay)
		for range 100 {
			d := serializableRetryDelay(attempt)
			require.GreaterOrEqual(t, d, time.Duration(0))
			require.LessOrEqual(t, d, upper)
		}
	}
}
