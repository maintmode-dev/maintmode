package token

import (
	"context"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
)

// maxPruneBatches caps how many batches one PruneRefreshTokens call runs, so a
// single sweep can never loop unbounded. The periodic job runs again on the
// next tick and drains whatever is left.
const maxPruneBatches = 100

// defaultPruneBatchLimit is the per-statement DELETE bound used when the caller
// passes a non-positive batchLimit, so the drain loop's "deleted < batchLimit"
// stop condition stays meaningful.
const defaultPruneBatchLimit = 1000

// defaultPruneRetention is how long past expires_at a row is kept when the
// caller passes a non-positive retention. It must stay equal to the processor's
// default and to the value shipped in deployment config.
//
// The coercion is the safety half, not a convenience: a non-positive retention
// is the only way to reach a cutoff at or after now, which would delete rows
// that have not expired yet -- live sessions, and the rotated-out rows reuse
// detection reads to recognize a replayed token.
const defaultPruneRetention = 24 * time.Hour

// PruneRefreshTokens deletes refresh-token rows whose expires_at is older than
// the retention window, in bounded batches, until a batch comes back short or
// the per-call batch cap is hit.
//
// The cutoff is computed once, so every batch targets the same instant -- rows
// expiring mid-sweep wait for the next tick rather than shifting the boundary
// under the loop.
func (s *Service) PruneRefreshTokens(ctx context.Context, retention time.Duration, batchLimit int64) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AccessToken.PruneRefreshTokens")
	defer span.End()

	if batchLimit <= 0 {
		batchLimit = defaultPruneBatchLimit
	}
	if retention <= 0 {
		retention = defaultPruneRetention
	}

	cutoff := s.getNowF().Add(-retention)

	var total int64
	for range maxPruneBatches {
		deleted, err := s.tokensStore.PruneExpiredBefore(ctx, cutoff, batchLimit)
		if err != nil {
			xlog.Error(ctx, "failed to prune expired refresh tokens batch",
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

	// Logged unconditionally, a zero-row sweep included: the job has no metric,
	// so this line is the only evidence it ran.
	xlog.Info(ctx, "pruned expired refresh tokens",
		xfield.Int64("count", total),
		xfield.Time("cutoff", cutoff),
	)

	return nil
}
