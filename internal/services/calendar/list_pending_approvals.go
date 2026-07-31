package calendar

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/calendardto"
	"github.com/ruko1202/maintmode/internal/entity"
)

// ListPendingApprovals returns the draft maintenances awaiting one approver.
// Read-side projection: no calendar specifics (period bounds, the 90-day limit)
// apply here.
func (s *Service) ListPendingApprovals(
	ctx context.Context,
	filter *calendardto.PendingApprovalsFilter,
) (*calendardto.PendingApprovalsResult, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Calendar.ListPendingApprovals")
	defer span.End()

	maints, total, err := s.maintStore.ListPendingApprovals(ctx, filter)
	if err != nil {
		xlog.Error(ctx, "failed to list pending approvals", xfield.Error(err))
		// Returned unwrapped so callers keep matching the store's sentinels.
		return nil, err
	}

	return &calendardto.PendingApprovalsResult{
		Maintenances: lo.Map(maints, func(item *entity.Maintenance, _ int) *calendardto.Maintenance {
			return toListDTOMaint(item)
		}),
		Total: total,
	}, nil
}
