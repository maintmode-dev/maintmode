package dekrotator

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/secrets"
)

// Rotate re-wraps every DEK under the active KEK in a single transaction. For
// each row it unwraps with the DEK's current KEK, re-wraps with the active KEK,
// verifies the new envelope unwraps back to the same DEK, then persists it. Any
// failure aborts the whole transaction, so a partial rotation can never leave
// some DEKs re-wrapped and others not. It returns how many rows were re-wrapped.
//
// Rows already on the active KEK are skipped (not re-verified): rotation migrates
// keys, it is not a corruption scanner. So a corrupt row already stamped with the
// active KEK is left untouched — it surfaces when a consumer unwraps it, not here.
// Rows sealed under a KEK the keyring does not hold are likewise skipped (see the
// loop) and counted in skipped, not re-wrapped.
//
// Rotation is O(all DEKs): it loads every row and holds FOR UPDATE locks for the
// whole re-wrap loop in one transaction. This is fine for the current design (one
// shared DEK, so a handful of rows); it is not built for tens of thousands of
// DEKs, where the long-held locks / open transaction would matter.
//
// Rotations are serialized against each other by an advisory lock, so concurrent
// callers run one at a time rather than deadlocking on the FOR UPDATE scan; the
// loser of the race simply finds every row already on the active KEK and re-wraps
// nothing. Rotation is idempotent, so that is a no-op, not a lost update.
func (r *Rotator) Rotate(ctx context.Context) (rewrapped, skipped int, err error) {
	ctx, span := xlog.WithOperationSpan(ctx, "datakey.Rotate")
	defer span.End()

	activeKEKID := r.keyring.ActiveKEKID()

	err = r.txManager.WithinTx(ctx, func(ctx context.Context) error {
		// Serialize rotations before the scan below. ListForUpdate locks every row
		// in the table, and two concurrent scans deadlock on those tuple locks
		// (see AdvisoryLockKeyDEKRotation) — which happens whenever more than one
		// replica boots at once, since each rotates at startup. Blocking here
		// instead means the second rotation waits and then finds the work already
		// done, re-wrapping nothing.
		if lockErr := r.store.LockRotation(ctx); lockErr != nil {
			return lockErr
		}

		keys, listErr := r.store.ListForUpdate(ctx)
		if listErr != nil {
			return listErr
		}

		for _, dk := range keys {
			if dk.KEKID == activeKEKID {
				continue // already wrapped by the active KEK
			}
			// A DEK sealed under a KEK the keyring doesn't hold cannot be re-wrapped
			// (we can't unwrap it), and it isn't rotation's job to: rotation migrates
			// DEKs between KEKs it holds, it is not a corruption scanner. Such a row is
			// orphaned or foreign (e.g. left by another tenant on a shared dev DB); skip
			// it with a warning. If it protects a live secret, that surfaces loudly at
			// resolve time (unwrap hard-errors there), not by aborting every startup.
			if !r.keyring.Knows(dk.KEKID) {
				xlog.Warn(ctx, "skipping dek sealed under an unknown kek during rotation",
					xfield.String("dek_id", dk.ID.String()),
					xfield.String("kek_id", dk.KEKID),
				)
				skipped++
				continue
			}
			if rewrapErr := r.rewrapOne(ctx, dk); rewrapErr != nil {
				return rewrapErr
			}
			rewrapped++
		}

		return nil
	})
	if err != nil {
		// rewrapped/skipped are meaningless on rollback; report zero to avoid
		// implying partial progress that the transaction just undid.
		xlog.Error(ctx, "kek rotation failed, rolled back", xfield.Error(err))
		return 0, 0, err
	}

	xlog.Info(ctx, "kek rotation complete",
		xfield.String("active_kek_id", activeKEKID),
		xfield.Int("rewrapped", rewrapped),
		xfield.Int("skipped", skipped),
	)
	return rewrapped, skipped, nil
}

// rewrapOne unwraps a DEK with its current KEK, re-wraps it under the active KEK,
// verifies the new envelope unwraps back to the same DEK, and persists it. The
// verify step catches a bad new KEK or a corrupt wrap while the transaction can
// still roll back — before the original envelope is overwritten.
func (r *Rotator) rewrapOne(ctx context.Context, dk *entity.DataKey) error {
	dek, err := r.keyring.UnwrapDEK(dk.EncryptedDEK, dk.KEKID)
	if err != nil {
		return fmt.Errorf("rotate: unwrap dek %s: %w", dk.ID, err)
	}

	newWrapped, newKEKID, err := r.keyring.WrapDEK(dek)
	if err != nil {
		return fmt.Errorf("rotate: wrap dek %s: %w", dk.ID, err)
	}

	check, err := r.keyring.UnwrapDEK(newWrapped, newKEKID)
	if err != nil {
		return fmt.Errorf("rotate: verify unwrap dek %s: %w", dk.ID, err)
	}
	// Functional equivalence, not byte equality: the unwrap re-serializes the
	// keyset proto, which is not contractually canonical.
	if err := secrets.EquivalentDEKs(dek, check); err != nil {
		return fmt.Errorf("rotate: verify mismatch for dek %s: %w", dk.ID, err)
	}

	if err := r.store.UpdateWrap(ctx, dk.ID, newWrapped, newKEKID); err != nil {
		return fmt.Errorf("rotate: persist dek %s: %w", dk.ID, err)
	}
	return nil
}
