package otp

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
// stop condition stays meaningful.
const defaultPruneBatchLimit = 1000

// defaultPruneRetention is the age threshold used when the caller passes a
// non-positive retention.
//
// It is 24h, NOT the 365 days its invitation counterpart uses, and the direction
// of safety is why. A one-time code lives minutes, so a day is already a wide
// margin for looking into a failed sign-in; a year-long fallback would silently
// disable the sweep and let code digests and session nonces pile up -- the exact
// thing this job exists to prevent. It must stay equal to the value shipped in
// deployment config, so that an unset retention behaves like the configured one.
const defaultPruneRetention = 24 * time.Hour

// Prune deletes one-time codes whose expires_at is older than the retention
// window, in bounded batches. Consumed and merely-expired codes leave through
// this same sweep -- consumed_at is not part of the predicate -- and password
// credentials are never eligible.
//
// batchLimit bounds one DELETE so the per-statement lock footprint stays small;
// Prune loops batches until one comes back short (the table is drained for this
// cutoff) or the per-call batch cap is hit. Both tunables come from the cron task
// payload (derived from config).
//
// The cutoff is computed once at call time so every batch in this sweep targets
// the same instant -- rows aging past the threshold mid-sweep wait for the next
// tick rather than shifting the boundary under the loop.
//
// SAFETY. Coercing a non-positive retention is not cosmetic. The per-code attempt
// ceiling is enforced by the row's continued existence: claimSlot refuses to
// issue a new code while it finds a live row with the attempts exhausted, so
// deleting such a row would hand back a fresh code with a fresh counter and turn
// "five attempts per code" into "five attempts per code, unlimited codes". A
// non-positive retention is the only way to reach a cutoff at or after now, and
// hence the only way to make a live row eligible -- so the coercion below is what
// closes that hole. Every positive retention is safe however small, because a row
// is eligible only once its expires_at is already past.
func (s *Service) Prune(ctx context.Context, retention time.Duration, batchLimit int64) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.OTP.Prune")
	defer span.End()

	if batchLimit <= 0 {
		batchLimit = defaultPruneBatchLimit
	}
	if retention <= 0 {
		retention = defaultPruneRetention
	}

	cutoff := xtime.UTCNow().Add(-retention)

	// Unreachable given the coercion above, and kept deliberately: it is a guard
	// against a future edit narrowing that fallback or reordering these steps,
	// not against any input reachable today. Do not "fix" the fallback to make
	// this branch testable -- refusing a zero retention instead of defaulting it
	// would break the documented "unset means default" contract.
	if !cutoff.Before(xtime.UTCNow()) {
		xlog.Error(ctx, "refusing to prune one-time codes with a cutoff that is not in the past",
			xfield.Time("cutoff", cutoff),
			xfield.Duration("retention", retention),
		)
		return nil
	}

	var total int64
	for range maxPruneBatches {
		deleted, err := s.store.PruneOTPExpiredBefore(ctx, cutoff, batchLimit)
		if err != nil {
			xlog.Error(ctx, "failed to prune expired one-time codes batch",
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

	// Logged unconditionally, including a zero-row sweep -- a knowing divergence
	// from the invitation sweep, which logs only when it deleted something. This
	// job has no metric, so the line is the only evidence it ran: without it a
	// cron that stopped firing looks exactly like a cron with nothing to do.
	xlog.Info(ctx, "pruned expired one-time codes",
		xfield.Int64("count", total),
		xfield.Time("cutoff", cutoff),
	)

	return nil
}
