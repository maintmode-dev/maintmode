package conflicts

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

// GetActualConflicts returns the maintenances that actually ran alongside this
// one, with their resources resolved the same way the live path resolves them.
//
// The second return value reports that the store truncated the neighbor list.
func (s *Service) GetActualConflicts(
	ctx context.Context,
	cmd *entity.ActualConflictQueryCmd,
) ([]*entity.ConflictWithResources, bool, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Conflicts.GetActualConflicts")
	defer span.End()

	conflicts, truncated, err := s.conflictsStore.ActualConflictedMaints(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "failed to get actual conflicted maints", xfield.Error(err))
		return nil, false, err
	}

	if truncated {
		xlog.Warn(ctx, "actual conflicts truncated; the reported list is incomplete",
			xfield.String("maintID", cmd.MaintID.String()),
		)
	}

	withResources, err := s.attachResources(ctx, conflicts)
	if err != nil {
		return nil, false, err
	}

	return withResources, truncated, nil
}

// GetApprovalSnapshot returns the conflict set frozen at approval, as conflicts
// in their own right rather than as a marker over a live set.
//
// This serves the one terminal case with no factual window: a maintenance that
// never started. Nothing ran, so there is no factual overlap to report, but
// "what did we see when we approved this" is still a real question — and the
// snapshot is the only record of it. A maintenance canceled straight from draft
// was never approved and correctly yields nothing.
//
// Resources are re-resolved from the live tables instead of read from the
// stored rows. Those rows arrive verbatim in the approve request body, and the
// approve gate hashes only each neighbor's SHARED subset, so anything outside
// it is client-supplied data that nothing validates — not what an incident
// review should be shown when the authoritative value is one query away. Title
// and scope already come from a server-side join, so after this no
// client-supplied value reaches the card.
//
// Unlike MarkApprovalState, a read failure here propagates: the snapshot is
// this branch's entire content, so degrading would render a maintenance that
// had conflicts as one that had none.
func (s *Service) GetApprovalSnapshot(
	ctx context.Context,
	maintID uuid.UUID,
) ([]*entity.ConflictWithResources, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Conflicts.GetApprovalSnapshot")
	defer span.End()

	snapshot, err := s.conflictSnapshotsStore.GetSnapshots(ctx, maintID)
	if err != nil {
		xlog.Error(ctx, "failed to read conflict snapshot", xfield.Error(err))
		return nil, err
	}

	resolved, err := s.attachResources(ctx, lo.Map(snapshot,
		func(item *entity.ConflictWithResources, _ int) *entity.Conflict {
			return item.Conflict
		}),
	)
	if err != nil {
		return nil, err
	}

	// True by construction: every entry here is, by definition, what the
	// approver was shown. Set explicitly so the read path does not have to
	// special-case this branch when it sorts.
	for _, conflict := range resolved {
		conflict.KnownAtApproval = true
	}
	entity.SortConflicts(resolved)

	return resolved, nil
}

// attachResources resolves each conflict's own resource set, which is
// deliberately the full set the neighboring maintenance touches rather than
// the subset it shares with the viewer (see ConflictedResources).
func (s *Service) attachResources(
	ctx context.Context,
	conflicts []*entity.Conflict,
) ([]*entity.ConflictWithResources, error) {
	resources, err := s.ConflictedResources(ctx, &entity.ConflictResourcesQueryCmd{
		ConflictedMaintIDs: lo.Map(conflicts, func(item *entity.Conflict, _ int) uuid.UUID {
			return item.MaintenanceID
		}),
	})
	if err != nil {
		xlog.Error(ctx, "failed to get conflicted resources", xfield.Error(err))
		return nil, err
	}

	return lo.Map(conflicts, func(item *entity.Conflict, _ int) *entity.ConflictWithResources {
		return &entity.ConflictWithResources{
			Conflict:  item,
			Resources: lo.ValueOr(resources, item.MaintenanceID, []uuid.UUID{}),
		}
	}), nil
}
