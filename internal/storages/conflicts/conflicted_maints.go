package conflicts

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func (s *Store) ConflictedMaints(ctx context.Context, cmd *entity.ConflictQueryCmd) ([]*entity.Conflict, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Conflicts.AddResources")
	defer span.End()

	/* slq like this
	SELECT
	    m.id, m.title
	     , m.scope
	     , tstzrange(
	        GREATEST(lower(m.planned_period), lower(tstzrange(:my_maint_period))),
	        LEAST(upper(m.planned_period),   upper(tstzrange(:my_maint_period)))
	       )                                     AS overlap_period
	FROM maintenances m
	WHERE
	  -- 1. только активные
	    m.status IN ('planned', 'in_progress')

	  -- 2. не сам с собой
	  AND m.id <> :my_maint_id

	  -- 3. пересечение по времени
	  AND m.planned_period && :my_maint_period

	-- 4. зона влияния
	  AND (
	    	-- my_maint global → конфликт со всеми
	    	:my_maint_scope = 'global'

	        -- existing global → конфликт со всеми
	        OR m.scope = 'global'

	        -- оба resource-bound → проверяем ресурсы
	        OR (
	        	:my_maint_scope = 'resource'
	            AND m.scope = 'resource'
	            AND EXISTS (
					SELECT 1 FROM maintenance_resources mr_d
					WHERE mr_m.maintenance_id = m.id
					  AND mr.resource_id = ANY(:my_maint_resource_ids)
				)
	        )
	    )
	;
	*/
	planetPeriod := xtime.ToPgRange(cmd.PlannedPeriod)
	planetPeriodTSTZRange := postgres.TSTZ_RANGE(
		postgres.TimestampzT(planetPeriod.Lower.Time),
		postgres.TimestampzT(planetPeriod.Upper.Time),
		postgres.String("[)"),
	)

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

	stmt := table.Maintenances.
		SELECT(
			table.Maintenances.ID.AS("conflict.maint_id"),
			table.Maintenances.Scope.AS("conflict.scope"),
			table.Maintenances.Title.AS("conflict.title"),
			postgres.TSTZ_RANGE(
				postgres.TimestampzExp(postgres.GREATEST(
					table.Maintenances.PlannedPeriod.LOWER_BOUND(), planetPeriodTSTZRange.LOWER_BOUND(),
				)),
				postgres.TimestampzExp(postgres.LEAST(
					table.Maintenances.PlannedPeriod.UPPER_BOUND(), planetPeriodTSTZRange.UPPER_BOUND(),
				)),
			).AS("conflict.overlap_period"),
		).
		WHERE(
			postgres.AND(
				table.Maintenances.ID.NOT_EQ(postgres.UUID(cmd.MaintID)),
				table.Maintenances.Status.IN(
					postgres.String(string(entity.MaintenanceStatusPlanned)),
					postgres.String(string(entity.MaintenanceStatusInProgress)),
				),
				table.Maintenances.PlannedPeriod.OVERLAP(planetPeriodTSTZRange),
				postgres.OR(
					// has overlap with `global` scope maintenances
					overlapByGlobalScope,
					// check overlap with `resource` scope maintenances
					overlapByResourcesStmt,
				),
			),
		)

	conflicts := make([]*Conflict, 0)
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), &conflicts)
	if err != nil {
		return nil, err
	}

	return lo.Map(conflicts, func(item *Conflict, _ int) *entity.Conflict {
		return fromDBConflict(item)
	}), nil
}
