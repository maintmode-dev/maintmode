package conflicts

import (
	"context"
	"fmt"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// ActualConflictsLimit caps how many neighbors one factual query reports.
//
// Removing the status filter (see below) removes what keeps the live query
// small in practice: few maintenances are planned or in progress at any moment,
// but the number that have EVER run grows without bound. The cap is far above
// any plausible real overlap and exists so a long global-scope maintenance
// cannot feed an unbounded list into the resource fan-outs downstream.
const ActualConflictsLimit = 200

// ActualConflictedMaints reports who actually worked alongside a finished
// maintenance, matching actual_period against actual_period.
//
// This answers a different question than ConflictedMaints. That one filters
// neighbors to the active statuses because it gates an approval: a canceled
// maintenance cannot break something you are about to start. Here the
// maintenance has already run, so a neighbor's final status is irrelevant —
// what matters is whether it was working at the same time.
//
// The absence of a status predicate is the entire point. A maintenance that ran
// without ever being approved is invisible to the live query (wrong status) and
// to the approval snapshot (it was not there to be seen), so this is the only
// place it can surface — which is exactly what an incident review is looking
// for. Do not "restore" a status filter here.
//
// The second return value reports truncation.
func (s *Store) ActualConflictedMaints(
	ctx context.Context,
	cmd *entity.ActualConflictQueryCmd,
) ([]*entity.Conflict, bool, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Conflicts.ActualConflictedMaints")
	defer span.End()

	// An open window is an invariant violation, not an input to tolerate: the
	// caller reaches this query only for a terminal maintenance with a closed
	// period, and routes one without one to the approval snapshot instead. Left
	// unchecked it would silently widen the comparison to an unbounded range.
	if cmd.ActualPeriod.IsOpen() {
		return nil, false, fmt.Errorf("actual period must be closed, got open-ended from %s",
			cmd.ActualPeriod.Start)
	}

	/* sql like this
	SELECT
	    m.id, m.title, m.scope
	     , tstzrange(
	        GREATEST(lower(m.actual_period), lower(tstzrange(:my_actual_period))),
	        LEAST(upper(m.actual_period),    upper(tstzrange(:my_actual_period)))
	       )                                     AS overlap_period
	FROM maintenances m
	WHERE
	    m.id <> :my_maint_id
	  -- actually ran: a maintenance that never started conflicts with nobody
	  AND m.actual_period IS NOT NULL
	  -- real time overlap; deliberately NO status predicate
	  AND m.actual_period && :my_actual_period
	  -- impact zone, same rule as the live query
	  AND ( :my_maint_scope = 'global' OR m.scope = 'global' OR ( ... shared resource ... ) )
	ORDER BY lower(overlap), m.id
	LIMIT :limit + 1
	*/
	actualPeriod := xtime.ToPgRange(cmd.ActualPeriod)
	actualPeriodTSTZRange := postgres.TSTZ_RANGE(
		postgres.TimestampzT(actualPeriod.Lower.Time),
		postgres.TimestampzT(actualPeriod.Upper.Time),
		postgres.String("[)"),
	)

	// Deliberately a copy of the scope block in ConflictedMaints rather than a
	// shared helper. That query is what the approve gate fingerprints inside a
	// SERIALIZABLE transaction, and an error introduced while refactoring it
	// would reject valid approvals; duplicating ~20 lines keeps the gate's SQL
	// byte-identical to what shipped. Note the empty-ResourceIDs behavior both
	// copies share: a resource-scoped maintenance holding no resources matches
	// global neighbors only.
	overlapByResourcesStmt := postgres.Bool(false)
	if len(cmd.ResourceIDs) > 0 {
		overlapByResourcesStmt = postgres.AND(
			postgres.String(string(cmd.Scope)).EQ(postgres.String(string(entity.MaintenanceScopeResources))),
			table.Maintenances.Scope.EQ(postgres.String(string(entity.MaintenanceScopeResources))),
			postgres.EXISTS(
				table.MaintenanceResources.
					SELECT(postgres.Int(1)).
					WHERE(
						postgres.AND(
							table.MaintenanceResources.MaintenanceID.EQ(table.Maintenances.ID),
							table.MaintenanceResources.ResourceID.EQ(postgres.ANY(
								postgres.ARRAY(uuidsToPgUUID(cmd.ResourceIDs)...),
							)),
						),
					),
			),
		)
	}

	overlapByGlobalScope := postgres.OR(
		postgres.String(string(cmd.Scope)).EQ(postgres.String(string(entity.MaintenanceScopeGlobal))),
		table.Maintenances.Scope.EQ(postgres.String(string(entity.MaintenanceScopeGlobal))),
	)

	// Both bounds are clamped to our own window. A neighbor still in progress
	// has an unbounded upper bound, and reporting that verbatim would not merely
	// look odd: an absent bound arrives as a nil Period.End, which fromDBConflict
	// turns into the ZERO time, rendering an overlap ending in year 1 with no
	// error anywhere.
	//
	// GREATEST and LEAST ignore NULL arguments, so an absent bound on either
	// side collapses to our own — no COALESCE needed, on either side. That is
	// the whole guarantee, and it does not lean on the schema: the table's CHECK
	// permits an unbounded LOWER bound too, since `lower(p) < upper(p)` yields
	// NULL rather than false when the lower bound is absent, and a CHECK accepts
	// NULL. Such a row is writable today, and this projection handles it.
	overlapStart := postgres.TimestampzExp(postgres.GREATEST(
		table.Maintenances.ActualPeriod.LOWER_BOUND(), actualPeriodTSTZRange.LOWER_BOUND(),
	))
	overlapPeriod := postgres.TSTZ_RANGE(
		overlapStart,
		postgres.TimestampzExp(postgres.LEAST(
			table.Maintenances.ActualPeriod.UPPER_BOUND(), actualPeriodTSTZRange.UPPER_BOUND(),
		)),
	)

	stmt := table.Maintenances.
		SELECT(
			table.Maintenances.ID.AS("conflict.maint_id"),
			table.Maintenances.Scope.AS("conflict.scope"),
			table.Maintenances.Title.AS("conflict.title"),
			overlapPeriod.AS("conflict.overlap_period"),
		).
		WHERE(
			postgres.AND(
				table.Maintenances.ID.NOT_EQ(postgres.UUID(cmd.MaintID)),
				// Defense in depth, not the working filter: && already yields
				// NULL for a NULL range, so a maintenance that never ran is
				// excluded by the overlap test alone. Kept because it states the
				// intent at the predicate level and would become load-bearing if
				// the overlap were ever restructured (an OR-joined or COALESCEd
				// form would let NULL periods back in). Note this means the
				// "never started" test cannot fail on its removal.
				table.Maintenances.ActualPeriod.IS_NOT_NULL(),
				table.Maintenances.ActualPeriod.OVERLAP(actualPeriodTSTZRange),
				postgres.OR(
					// has overlap with `global` scope maintenances
					overlapByGlobalScope,
					// check overlap with `resource` scope maintenances
					overlapByResourcesStmt,
				),
			),
		).
		// Ordered by the same clamped start the projection reports, so truncation
		// drops the latest-starting neighbors rather than an arbitrary subset.
		ORDER_BY(
			overlapStart.ASC(),
			table.Maintenances.ID.ASC(),
		).
		// One row past the cap: LIMIT alone cannot tell "exactly at the cap"
		// from "far more than the cap". Same probe maintenances.GetMaints uses.
		LIMIT(ActualConflictsLimit + 1)

	rows := make([]*Conflict, 0, ActualConflictsLimit+1)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &rows); err != nil {
		return nil, false, err
	}

	truncated := len(rows) > ActualConflictsLimit
	if truncated {
		rows = rows[:ActualConflictsLimit]
	}

	return lo.Map(rows, func(item *Conflict, _ int) *entity.Conflict {
		return fromDBConflict(item)
	}), truncated, nil
}
