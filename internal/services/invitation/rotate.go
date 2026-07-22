package invitation

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// maxRotateBatches caps how many batches one Rotate call runs, so a single sweep
// can never loop unbounded. The periodic job runs again on the next tick, so any
// leftover expired-pending rows are flipped then; this only bounds per-invocation
// work.
const maxRotateBatches = 100

// defaultRotateBatchLimit is the per-statement UPDATE bound used when the caller
// passes a non-positive batchLimit, so the drain loop's "flipped < batchLimit"
// stop condition stays meaningful. Mirrors the producer-side default in
// invitationrotateprocessor.
const defaultRotateBatchLimit = 1000

// Rotate flips pending invitations whose expires_at has passed to the persisted
// 'expired' status in bounded batches. Until now 'expired' was only derived on
// read; persisting it makes the stored status honest and frees each rotated
// email's active-pending slot (the row leaves the partial-unique pending index).
//
// batchLimit bounds one UPDATE so the per-statement lock stays small; Rotate
// loops batches until one comes back short (nothing left past the boundary) or
// the per-call batch cap is hit. The boundary is process time, captured once so
// every batch in this sweep targets the same instant — rows expiring mid-sweep
// wait for the next tick rather than shifting the boundary under the loop.
func (s *Service) Rotate(ctx context.Context, batchLimit int64) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Invitation.Rotate")
	defer span.End()

	if batchLimit <= 0 {
		batchLimit = defaultRotateBatchLimit
	}

	now := xtime.UTCNow()

	var total int64
	for range maxRotateBatches {
		flipped, err := s.store.ExpireOlderThan(ctx, now, batchLimit)
		if err != nil {
			xlog.Error(ctx, "failed to rotate expired invitations batch",
				xfield.Time("boundary", now),
				xfield.Int64("rotatedSoFar", total),
				xfield.Error(err),
			)
			return err
		}

		total += flipped
		if flipped < batchLimit {
			break
		}
	}

	if total > 0 {
		xlog.Info(ctx, "rotated expired invitations",
			xfield.Int64("count", total),
			xfield.Time("boundary", now),
		)
	}

	return nil
}
