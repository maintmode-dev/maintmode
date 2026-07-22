package invitation

import (
	"context"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// maxPruneBatches caps how many batches one Prune call runs, so a single sweep
// can never loop unbounded. The periodic job runs again on the next tick, so any
// leftover eligible rows are drained then; this only bounds per-invocation work.
const maxPruneBatches = 100

// defaultPruneBatchLimit is the per-statement DELETE bound used when the caller
// passes a non-positive batchLimit, so the drain loop's "deleted < batchLimit"
// stop condition stays meaningful. Mirrors the producer-side default in
// invitationpruneprocessor.
const defaultPruneBatchLimit = 1000

// defaultPruneRetention is the age threshold used when the caller passes a
// non-positive retention. A retention <= 0 would push the cutoff to or past now
// (a negative value into the future), making every terminal row eligible and
// wiping invite history — so a config typo must fail safe to a conservative
// window rather than mass-delete. Mirrors the producer-side default in
// invitationpruneprocessor.
const defaultPruneRetention = 365 * 24 * time.Hour

// Prune deletes terminal invitations (expired/accepted/revoked) whose created_at
// is older than the retention window in bounded batches. pending rows are never
// deleted — an expired-but-not-yet-rotated invite leaves via rotation first.
//
// batchLimit bounds one DELETE so the per-statement lock stays small; Prune loops
// batches until one comes back short (the table is drained for this cutoff) or
// the per-call batch cap is hit. Both tunables come from the cron task payload
// (derived from config).
//
// The cutoff is computed once at call time so every batch in this sweep targets
// the same instant — rows aging past the threshold mid-sweep wait for the next
// tick rather than shifting the boundary under the loop.
func (s *Service) Prune(ctx context.Context, retention time.Duration, batchLimit int64) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Invitation.Prune")
	defer span.End()

	if batchLimit <= 0 {
		batchLimit = defaultPruneBatchLimit
	}
	if retention <= 0 {
		retention = defaultPruneRetention
	}

	cutoff := xtime.UTCNow().Add(-retention)

	var total int64
	for range maxPruneBatches {
		deleted, err := s.store.PruneTerminalOlderThan(ctx, cutoff, batchLimit)
		if err != nil {
			xlog.Error(ctx, "failed to prune terminal invitations batch",
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
		xlog.Info(ctx, "pruned terminal invitations",
			xfield.Int64("count", total),
			xfield.Time("cutoff", cutoff),
		)
	}

	return nil
}
