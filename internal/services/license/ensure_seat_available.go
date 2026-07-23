package license

import (
	"fmt"

	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// EnsureSeatAvailable rejects granting a seat-occupying role when the org has
// reached its licensed seats_purchased. Call inside the mutation transaction,
// after the target row's FOR UPDATE and BEFORE the role change is persisted, so
// the fresh count excludes the in-flight grant (occupied+1 models it). No-op
// without a license (self-hosted) or when seats_purchased is unknown (Console
// has not reported yet). Pure cap check — the caller decides whether a real
// non-seat→seat transition is happening before calling this.
func (s *Service) EnsureSeatAvailable(ctx context.Context) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.License.EnsureSeatAvailable")
	defer span.End()

	lic := s.License(ctx) // TTL cache; a slightly stale limit is acceptable
	if lic == nil || lic.SeatsPurchased == nil {
		return nil // unlimited: no license, or no reported cap yet
	}

	// Serialize the count-then-write decision across concurrent seat grants —
	// mirrors ensureNotLastActiveAdmin. Released on commit/rollback.
	if err := dbtx.AdvisoryXactLock(ctx, dbtx.AdvisoryLockKeySeatMutations); err != nil {
		return fmt.Errorf("lock seat mutations: %w", err)
	}

	active, err := s.users.ListActiveRoles(ctx) // fresh, in-tx — not the cache
	if err != nil {
		return fmt.Errorf("list active roles: %w", err)
	}
	pending, err := s.invitations.ListPendingRoles(ctx)
	if err != nil {
		return fmt.Errorf("list pending roles: %w", err)
	}

	occupied := entity.BucketSeats(active, pending).SeatsOccupied()
	if occupied+1 > *lic.SeatsPurchased {
		// Observability (B1): the guard firing is invisible to ops otherwise — a
		// wrongly-firing cap (stale/too-low seats_purchased from Console) presents
		// to the admin as a generic 403. Warn so an operator can tell the cap is
		// working, and distinguish a real cap from a mis-fire.
		xlog.Warn(ctx, "seat cap reached, rejecting grant",
			xfield.Int64("occupied", occupied),
			xfield.Int64("seats_purchased", *lic.SeatsPurchased),
		)
		return fmt.Errorf("all %d of %d seats are in use: %w",
			occupied, *lic.SeatsPurchased, apperr.ErrSeatsLimitExceeded)
	}
	return nil
}
