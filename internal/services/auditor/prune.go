package auditor

import (
	"context"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// maxPruneBatches caps how many batches one Prune call runs, so a single sweep
// can never loop unbounded if the table is enormous or rows keep arriving. The
// periodic job runs again on the next tick, so leftover expired rows are drained
// then; this only bounds the work done per invocation.
const maxPruneBatches = 100

// defaultBatchLimit is the per-statement DELETE bound used when the caller passes
// a non-positive batchLimit, so the drain loop's "deleted < batchLimit" stop
// condition stays meaningful. Mirrors the producer-side default in
// auditpruneprocessor.
const defaultBatchLimit = 1000

// Prune deletes audit_log rows older than the retention window in bounded
// batches. retention is the age threshold (e.g. 365d): every row whose created_at
// is before now-retention is eligible. batchLimit bounds one DELETE so the
// per-statement lock stays small on a table that may have grown large before
// retention was switched on; Prune loops batches until one comes back short (the
// table is drained for this cutoff) or the per-call batch cap is hit. Both tunables
// come from the cron task payload (derived from config), keeping this method free
// of scheduling policy.
//
// The cutoff is computed once at call time so every batch in this sweep targets
// the same instant — rows aging past the threshold mid-sweep wait for the next
// tick rather than shifting the boundary under the loop.
func (a *Auditor) Prune(ctx context.Context, retention time.Duration, batchLimit int64) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Auditor.Prune")
	defer span.End()

	// Guard the loop's termination condition: with batchLimit <= 0 the
	// "deleted < batchLimit" check below could never trip, so a bad config value
	// (the factory's cmp.Or only catches exactly zero) would otherwise run all
	// maxPruneBatches doing nothing. Fall back to the same default the factory uses.
	if batchLimit <= 0 {
		batchLimit = defaultBatchLimit
	}

	cutoff := xtime.UTCNow().Add(-retention)

	var total int64
	for range maxPruneBatches {
		deleted, err := a.store.PruneOlderThan(ctx, cutoff, batchLimit)
		if err != nil {
			xlog.Error(ctx, "failed to prune audit log batch",
				xfield.Time("cutoff", cutoff),
				xfield.Int64("prunedSoFar", total),
				xfield.Error(err),
			)
			return err
		}

		total += deleted
		if deleted < batchLimit {
			break
		}
	}

	if total > 0 {
		xlog.Info(ctx, "pruned expired audit log rows",
			xfield.Int64("count", total),
			xfield.Time("cutoff", cutoff),
		)
	}

	return nil
}
