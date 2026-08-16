package calendar

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/calendardto"
	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) GetConflicts(ctx context.Context, cmd *calendardto.ConflictQueryCmd) ([]*calendardto.Conflict, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Calendar.GetConflicts")
	defer span.End()

	conflicts, err := s.resolveConflicts(ctx, cmd)
	if err != nil {
		return nil, err
	}

	conflictResourcesM := lo.SliceToMap(conflicts, func(item *entity.ConflictWithResources) (uuid.UUID, []uuid.UUID) {
		return item.MaintenanceID, item.Resources
	})

	// Deduplicated before the lookup: neighbors routinely share resources, so the
	// flattened list repeats ids. GetResources matches on an id set and returns a
	// map, so duplicates would collapse anyway — this only stops us shipping a
	// needlessly long array to Postgres, and so changes no observable output.
	resourcesDetails, err := s.resolveResourceNames(ctx, lo.Uniq(lo.Flatten(lo.Values(conflictResourcesM))))
	if err != nil {
		xlog.Error(ctx, "failed to get resources details", xfield.Error(err))
		return nil, err
	}

	// The conflicts service already ordered these — unreviewed first, then by
	// overlap start, then by id. Keep that order.
	return lo.Map(conflicts, func(item *entity.ConflictWithResources, _ int) *calendardto.Conflict {
		resources := lo.ValueOr(conflictResourcesM, item.MaintenanceID, []uuid.UUID{})
		return &calendardto.Conflict{
			MaintenanceID:   item.MaintenanceID,
			Title:           item.Title,
			OverlapStart:    item.OverlapStart,
			OverlapEnd:      item.OverlapEnd,
			Scope:           item.Scope,
			KnownAtApproval: item.KnownAtApproval,
			Resources: lo.Map(resources, func(id uuid.UUID, _ int) *calendardto.MaintenanceResource {
				// fallback to "unknown resource" if the resource is not found
				resDetails := lo.ValueOr(resourcesDetails, id, &entity.ResourceDetails{Name: "unknown resource"})
				return &calendardto.MaintenanceResource{
					ID:   id,
					Name: resDetails.Name,
				}
			}),
		}
	}), nil
}

// resolveConflicts picks which question the card is answering, which depends on
// where the maintenance sits in its lifecycle:
//
//	draft / planned / in_progress    → plannedConflicts
//	terminal WITH a closed window    → factualConflicts
//	terminal WITHOUT one             → the approval snapshot. Nothing ran, so no
//	    factual overlap exists, but "what did we see when we approved" is still
//	    answerable. A draft canceled before approval has no snapshot and
//	    correctly reports nothing.
//
// The condition is a conjunction, and the obvious simplification of it is
// wrong: an in_progress maintenance has a non-nil (open) actual period, so
// keying on the period alone would route a live card onto the factual path.
//
// The IsOpen check is not redundant with IsTerminal either. Every terminal write
// path closes the period today — both CompleteMaint and CancelMaint call
// CloseActualPeriod, including the auto-cancel sweep — but this branch must not
// depend on that: write-path behavior is outside this change, and a terminal
// row with an open period would otherwise reach the factual query and trip its
// closed-period precondition, turning a data anomaly into a failed card.
// Degrading to the snapshot is the safe direction.
func (s *Service) resolveConflicts(
	ctx context.Context,
	cmd *calendardto.ConflictQueryCmd,
) ([]*entity.ConflictWithResources, error) {
	if !cmd.Status.IsTerminal() {
		return s.plannedConflicts(ctx, cmd)
	}

	if cmd.ActualPeriod == nil || cmd.ActualPeriod.IsOpen() {
		// Never ran, or ran but lost its window to a drifted clock. Either way
		// there is no factual overlap to compute, and the snapshot is the only
		// record of what was known.
		return s.conflictsSrv.GetApprovalSnapshot(ctx, cmd.MaintID)
	}

	return s.factualConflicts(ctx, cmd)
}

// plannedConflicts answers "may I work?" from the planned overlaps — the same
// set the approve gate reads.
func (s *Service) plannedConflicts(
	ctx context.Context,
	cmd *calendardto.ConflictQueryCmd,
) ([]*entity.ConflictWithResources, error) {
	conflicts, err := s.conflictsSrv.GetConflicts(ctx, &entity.ConflictQueryCmd{
		MaintID:       cmd.MaintID,
		PlannedPeriod: cmd.PlannedPeriod,
		Scope:         cmd.Scope,
		ResourceIDs:   cmd.ResourceIDs,
	})
	if err != nil {
		xlog.Error(ctx, "failed to get conflicts", xfield.Error(err))
		return nil, err
	}

	// Flags which of these the approver had already seen and fixes the order —
	// unreviewed first. Never errors: an unreadable snapshot degrades to
	// "nobody reviewed any of this" rather than failing the card.
	return s.conflictsSrv.MarkApprovalState(ctx, cmd.MaintID, conflicts), nil
}

// factualConflicts answers "what actually happened?" from the real working
// windows, so a neighbor's final status is irrelevant and unapproved work
// surfaces.
func (s *Service) factualConflicts(
	ctx context.Context,
	cmd *calendardto.ConflictQueryCmd,
) ([]*entity.ConflictWithResources, error) {
	// The truncation flag is deliberately dropped rather than overlooked: the
	// response carries no in-band field for it, so the service's WARN is the
	// signal. Surfacing it would mean an API change.
	conflicts, _, err := s.conflictsSrv.GetActualConflicts(ctx, &entity.ActualConflictQueryCmd{
		MaintID:      cmd.MaintID,
		ActualPeriod: lo.FromPtr(cmd.ActualPeriod),
		Scope:        cmd.Scope,
		ResourceIDs:  cmd.ResourceIDs,
	})
	if err != nil {
		xlog.Error(ctx, "failed to get actual conflicts", xfield.Error(err))
		return nil, err
	}

	// The flag carries more weight here than on a live card: an unflagged
	// neighbor ran alongside this maintenance without the approver ever seeing
	// it, which is what an incident review is hunting for. SortConflicts puts
	// those first. Note the flag cannot distinguish that from "this maintenance
	// was never approved at all" — see the spec's known limitations.
	return s.conflictsSrv.MarkApprovalState(ctx, cmd.MaintID, conflicts), nil
}

// resolveResourceNames returns a map of resource ID → name for the given IDs.
// Used for projections (e.g., conflict-resources from a fingerprint snapshot)
// where only IDs are known and a name is needed for display.
func (s *Service) resolveResourceNames(ctx context.Context, resourceIDs []uuid.UUID) (map[uuid.UUID]*entity.ResourceDetails, error) {
	if len(resourceIDs) == 0 {
		return map[uuid.UUID]*entity.ResourceDetails{}, nil
	}

	details, err := s.resourcesStore.GetResources(ctx, resourceIDs)
	if err != nil {
		return nil, err
	}

	return lo.SliceToMap(details, func(item *entity.ResourceDetails) (uuid.UUID, *entity.ResourceDetails) {
		return item.ID, item
	}), nil
}
