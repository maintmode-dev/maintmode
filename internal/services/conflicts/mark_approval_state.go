package conflicts

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// MarkApprovalState annotates live conflicts with whether the approver had
// already seen them, and orders the result so the unreviewed ones lead.
//
// Live conflicts are the spine: a conflict that existed at approval but has
// since disappeared is not reported at all. The on-call reads this before
// starting work, and a maintenance that was canceled cannot break anything;
// for a post-incident review the same holds, since only the maintenances that
// actually ran alongside matter.
//
// A snapshot read failure degrades instead of propagating: the live set is what
// the reader opened the card for, and losing it over a failed audit hint would
// be the worse outcome. Everything is then reported as unreviewed — alarming
// rather than reassuring, which is the safe direction for a screen whose whole
// purpose is to warn.
func (s *Service) MarkApprovalState(
	ctx context.Context,
	maintID uuid.UUID,
	live []*entity.ConflictWithResources,
) []*entity.ConflictWithResources {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Conflicts.MarkApprovalState")
	defer span.End()

	if len(live) == 0 {
		return []*entity.ConflictWithResources{}
	}

	snapshot, err := s.conflictSnapshotsStore.GetSnapshots(ctx, maintID)
	if err != nil {
		// Swallowed, not propagated — see the doc comment. The log is the only
		// trace this leaves, so keep it clean: a canceled request or an expired
		// deadline means the reader is already gone and there is no card left to
		// degrade, and logging those at ERROR would bury the real faults among
		// closed browser tabs.
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			xlog.Error(ctx, "failed to read conflict snapshot", xfield.Error(err))
		}

		entity.SortConflicts(live)
		return live
	}

	entity.MarkKnownAtApproval(live, snapshot)
	entity.SortConflicts(live)

	return live
}
