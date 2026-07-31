package maintenances

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/calendardto"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// ListPendingApprovals returns a page of draft maintenances assigned to one
// approver, oldest first, together with the total matching the same filter.
func (s *Store) ListPendingApprovals(
	ctx context.Context,
	filter *calendardto.PendingApprovalsFilter,
) ([]*entity.Maintenance, int64, error) {
	// A nil approver means the caller lost the identity it filters by, so fail
	// loudly instead of querying. As written the clause would merely match no
	// rows, but this store's idiom elsewhere is "zero field = filter not
	// applied" (see filterToWhereExpr): were this predicate ever folded into
	// that shape, the same nil would widen the personal queue into every draft
	// in the organization. The guard holds the invariant regardless of how the
	// clause is built.
	if filter.ApproverUserID == uuid.Nil {
		return nil, 0, apperr.ErrNilApprover
	}

	ctx, span := xlog.WithOperationSpan(ctx, "store.Maintenances.ListPendingApprovals")
	defer span.End()

	where := pendingApprovalsWhereExpr(filter)

	total, err := s.countPendingApprovals(ctx, where)
	if err != nil {
		return nil, 0, err
	}

	stmt := table.Maintenances.
		SELECT(table.Maintenances.AllColumns).
		WHERE(where).
		ORDER_BY(
			table.Maintenances.CreatedAt.ASC(),
			table.Maintenances.ID.ASC(),
		).
		LIMIT(filter.Limit).
		OFFSET(filter.Offset)

	maints := make([]*model.Maintenances, 0, filter.Limit)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &maints); err != nil {
		return nil, 0, err
	}

	return lo.Map(maints, func(m *model.Maintenances, _ int) *entity.Maintenance {
		return fromDBMaintenance(m)
	}), total, nil
}

func (s *Store) countPendingApprovals(ctx context.Context, where postgres.BoolExpression) (int64, error) {
	stmt := table.Maintenances.
		SELECT(postgres.COUNT(postgres.STAR).AS("count")).
		WHERE(where)

	var dest struct {
		Count int64
	}
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &dest); err != nil {
		return 0, err
	}
	return dest.Count, nil
}

func pendingApprovalsWhereExpr(filter *calendardto.PendingApprovalsFilter) postgres.BoolExpression {
	return table.Maintenances.ApproverUserID.EQ(postgres.UUID(filter.ApproverUserID)).
		AND(table.Maintenances.Status.EQ(postgres.String(string(entity.MaintenanceStatusDraft))))
}
