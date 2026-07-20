package license

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// Refresh performs one heartbeat tick: collect the report fresh, send it,
// cache and publish the returned license.
//
// Failure semantics are deliberately asymmetric: any Console-side failure —
// transport error, non-200, malformed body — is fail-open: log and keep the
// current cached license, returning nil so the tick is "done". An admin-panel
// outage must never degrade a customer instance. Local failures (collecting
// the report, persisting the cache) are returned as errors — the next cron
// tick is the retry.
func (s *Service) Refresh(ctx context.Context) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.License.Refresh")
	defer span.End()

	usage, err := s.collectSeatUsage(ctx)
	if err != nil {
		xlog.Error(ctx, "failed to collect seat usage", xfield.Error(err))
		return err
	}

	lastActivity, err := s.audit.LastActivityAt(ctx)
	if err != nil {
		xlog.Error(ctx, "failed to get last activity time", xfield.Error(err))
		return err
	}

	license, err := s.client.Send(ctx, &entity.HeartbeatReport{
		SeatsUsed:      usage,
		LastActivityAt: lastActivity,
		Version:        s.version,
	})
	if err != nil {
		xlog.Error(ctx, "license heartbeat failed; keeping cached license", xfield.Error(err))
		return nil
	}

	fetchedAt := xtime.UTCNow()
	license.FetchedAt = &fetchedAt

	return s.licenseStore.Upsert(ctx, license)
}

// collectSeatUsage builds the per-role seat report for the heartbeat: active
// (non-blocked) users and live pending invitations, each counted once in the
// bucket of its highest role. Collected fresh at send time — a stale snapshot
// must never reach Console.
func (s *Service) collectSeatUsage(ctx context.Context) (entity.SeatUsage, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.License.CollectSeatUsage")
	defer span.End()

	activeRoles, err := s.users.ListActiveRoles(ctx)
	if err != nil {
		return entity.SeatUsage{}, err
	}

	pendingRoles, err := s.invitations.ListPendingRoles(ctx)
	if err != nil {
		return entity.SeatUsage{}, err
	}

	return entity.BucketSeats(activeRoles, pendingRoles), nil
}
