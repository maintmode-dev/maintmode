package maint

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
)

// validateApprover ensures the user being assigned as approver is eligible — an
// active reviewer/admin — by asking the auth service. An auth outage is
// propagated as-is (ErrAuthUnavailable => 503); an ineligible approver becomes
// ErrApproverNotEligible (=> 400). Called on the assignment paths (create and
// update), outside the DB transaction so the S2S call does not hold locks.
func (s *Service) validateApprover(ctx context.Context, approverID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.validateApprover")
	defer span.End()

	eligible, err := s.approverValidator.IsEligibleApprover(ctx, approverID)
	if err != nil {
		xlog.Error(ctx, "failed to validate approver", xfield.Error(err))
		return err
	}
	if !eligible {
		xlog.Error(ctx, "approver is not eligible", xfield.String("approver_id", approverID.String()))
		return fmt.Errorf("%w: %s", apperr.ErrApproverNotEligible, approverID)
	}

	return nil
}
