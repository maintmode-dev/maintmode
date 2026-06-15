package dbtx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
)

const (
	// serializableMaxAttempts caps how many times WithinSerializableTx replays a
	// transaction that lost a serialization conflict (40001). SERIALIZABLE detects
	// predicate-level conflicts, so an approve can lose not only to a racing
	// approve of the same window but to any concurrent approve/create whose period
	// overlaps the conflict query — bursts of them can abort several rounds in a
	// row. The path is human-driven and low-QPS, so a generous bound is cheap and
	// keeps legitimate operations from surfacing a spurious 409 under load;
	// exhausting it maps to a retryable 409 at the API.
	serializableMaxAttempts = 10

	// serializableRetryBaseDelay is the first backoff step between retries. Each
	// subsequent attempt doubles it (capped by serializableRetryMaxDelay) and
	// adds full jitter, so racing transactions don't re-collide in lockstep.
	serializableRetryBaseDelay = 5 * time.Millisecond

	// serializableRetryMaxDelay caps a single backoff step so total wait stays
	// bounded for a synchronous request path (worst case ~ attempts * cap).
	serializableRetryMaxDelay = 40 * time.Millisecond
)

type TxManager struct {
	db       *sqlx.DB
	commit   func(ctx context.Context, tx *sqlx.Tx) error
	rollback func(ctx context.Context, tx *sqlx.Tx) error
	// sleep backs off between serialization retries. Injectable so tests don't
	// spend real wall-clock time; defaults to time.Sleep.
	sleep func(time.Duration)
}

func NewTxManager(db *sqlx.DB) *TxManager {
	return &TxManager{
		db:       db,
		commit:   commit,
		rollback: rollback,
		sleep:    time.Sleep,
	}
}

// txConfig holds per-transaction settings assembled from TxOptions.
type txConfig struct {
	isolation sql.IsolationLevel
}

// TxOption tweaks a single WithinTx call. Unset options keep the defaults
// (READ COMMITTED), so existing callers are unaffected.
type TxOption func(*txConfig)

// WithIsolation runs the transaction at the given isolation level. Use
// sql.LevelSerializable for read-modify-write flows whose correctness depends on
// a stable view across the transaction (e.g. conflict detection on approve);
// pair it with WithinSerializableTx so lost-conflict aborts are retried.
func WithIsolation(level sql.IsolationLevel) TxOption {
	return func(c *txConfig) { c.isolation = level }
}

// WithinSerializableTx runs fn in a SERIALIZABLE transaction, retrying from
// scratch when Postgres aborts it with serialization_failure (40001). It is the
// right wrapper for read-modify-write flows that must observe a stable set of
// rows across the transaction. When already nested in an outer transaction it
// cannot restart, so it runs once and lets the outer WithinTx own retries.
func (m *TxManager) WithinSerializableTx(ctx context.Context, fn func(ctx context.Context) error) error {
	ctx, span := xlog.WithOperationSpan(ctx, "txManager.WithinSerializableTx")
	defer span.End()

	if _, nested := TxFromContext(ctx); nested {
		return m.WithinTx(ctx, fn)
	}

	var err error
	for attempt := 1; attempt <= serializableMaxAttempts; attempt++ {
		err = m.WithinTx(ctx, fn, WithIsolation(sql.LevelSerializable))
		if err == nil || !ErrorIs(err, ErrPGSerializationFailure) {
			return err
		}

		// Last attempt: don't sleep, just surface the conflict.
		if attempt == serializableMaxAttempts {
			break
		}

		// Bail out before sleeping if the caller already went away, so we don't
		// back off only to run a doomed attempt.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		delay := serializableRetryDelay(attempt)
		xlog.Warn(ctx, "serializable transaction conflict, retrying",
			xfield.Int("attempt", attempt),
			xfield.Duration("backoff", delay))
		m.sleep(delay)
	}

	return err
}

// serializableRetryDelay computes the backoff for a given (1-based) attempt:
// exponential growth capped at serializableRetryMaxDelay, then full jitter in
// [0, cap] so concurrent retries spread out instead of re-colliding.
func serializableRetryDelay(attempt int) time.Duration {
	backoff := min(serializableRetryBaseDelay<<(attempt-1), serializableRetryMaxDelay)
	return time.Duration(rand.Int64N(int64(backoff) + 1))
}

func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error, opts ...TxOption) (err error) {
	ctx, span := xlog.WithOperationSpan(ctx, "txManager.WithinTx")
	defer span.End()

	cfg := txConfig{isolation: sql.LevelDefault}
	for _, opt := range opts {
		opt(&cfg)
	}

	// Reentrant: if a transaction is already active in the context, join it
	// instead of opening a second, independent one. Begin/commit/rollback stay
	// with the outermost WithinTx — a nested call only runs fn against the
	// existing tx and propagates its error so the outer call rolls back the
	// whole unit. This lets a service that wraps work in a transaction call
	// other transactional services and have them all commit atomically.
	//
	// There are no savepoints: a nested call cannot roll back just its own part
	// while the outer transaction continues. Every caller propagates the error
	// upward (none swallows it to "continue after a failed sub-step"), so this
	// matches existing usage; do not rely on partial rollback.
	if _, ok := TxFromContext(ctx); ok {
		// Joining an existing transaction: isolation was fixed when the outer
		// WithinTx opened it and cannot be changed mid-transaction, so any
		// isolation option here is intentionally ignored.
		return fn(ctx)
	}

	tx, beginTxErr := m.db.BeginTxx(ctx, &sql.TxOptions{Isolation: cfg.isolation})
	if beginTxErr != nil {
		return beginTxErr
	}

	ctx = context.WithValue(ctx, txKey{}, tx)

	defer func() {
		if recErr := recover(); recErr != nil {
			xlog.Error(ctx, "panic recovery when execute the transaction", xfield.Any("panic", recErr))
			err = errors.Join(err, fmt.Errorf("panic recovery: %v", recErr))
		}

		if err != nil {
			if rollbackErr := m.rollback(ctx, tx); rollbackErr != nil {
				err = errors.Join(err, rollbackErr)
				xlog.Error(ctx, "raise error when rollback the transaction", xfield.Error(err))
			}
		}
	}()

	if fnErr := fn(ctx); fnErr != nil {
		err = errors.Join(err, fnErr)
		xlog.Error(ctx, "raise error when execute the transaction", xfield.Error(err))
		return err
	}

	if commitErr := m.commit(ctx, tx); commitErr != nil {
		err = errors.Join(err, commitErr)
		xlog.Error(ctx, "raise error when commit the transaction", xfield.Error(err))
		return err
	}

	return err
}

func rollback(ctx context.Context, tx *sqlx.Tx) error {
	if err := tx.Rollback(); err != nil {
		xlog.Error(ctx, "failed to rollback the transaction", xfield.Error(err))
		return err
	}
	return nil
}

func commit(ctx context.Context, tx *sqlx.Tx) error {
	if err := tx.Commit(); err != nil {
		xlog.Error(ctx, "failed to commit the transaction", xfield.Error(err))
		return err
	}
	return nil
}
