package maint

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// validateApprover ensures the user being assigned as approver is eligible — an
// active reviewer/admin — by querying the auth user service. A store error is
// propagated as-is; an ineligible approver becomes ErrApproverNotEligible
// (=> 400). Called on the assignment paths (create and update), outside the DB
// transaction so the read does not hold locks.
//
// Eligibility is the "approver" composition: a user returned under (id, the
// eligible roles, active-only) filters is, by construction, eligible. The user
// service exposes only a neutral listing; the maintmode notion of "approver"
// lives here.
func (s *Service) validateApprover(ctx context.Context, approverID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Maint.validateApprover")
	defer span.End()

	res, err := s.users.ListUsers(ctx, &entity.ListUsersCmd{
		IDs:            []uuid.UUID{approverID},
		Roles:          entity.ApproverEligibleRoles,
		ExcludeBlocked: true,
		// A single id is checked; Limit must be > 0 or the store emits LIMIT 0
		// and returns no rows (ListUsers does not default Limit).
		Limit: 1,
	})
	if err != nil {
		xlog.Error(ctx, "failed to validate approver", xfield.Error(err))
		return err
	}
	if len(res.Users) == 0 {
		xlog.Error(ctx, "approver is not eligible", xfield.String("approver_id", approverID.String()))
		return fmt.Errorf("%w: %s", apperr.ErrApproverNotEligible, approverID)
	}

	return nil
}
