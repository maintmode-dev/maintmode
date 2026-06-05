package authgateway

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	authclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/auth"
)

// IsEligibleApprover reports whether id belongs to an active (non-blocked) user
// holding the reviewer or admin role — the eligibility rule for being assigned
// as a maintenance approver. It is a single S2S query constrained by id, the
// eligible roles, and active=true: auth is the source of truth for both the
// active state and the roles, so an id returned under these filters is, by
// construction, an eligible approver. An empty result means "not eligible".
// Transport/status failures are wrapped in apperr.ErrAuthUnavailable, matching
// the sibling gateway methods.
func (s *Gateway) IsEligibleApprover(ctx context.Context, id uuid.UUID) (bool, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "gateway.Auth.IsEligibleApprover")
	defer span.End()

	active := true
	ids := []string{id.String()}
	roles := lo.Map(entity.ApproverEligibleRoles, func(r entity.Role, _ int) string {
		return string(r)
	})
	params := &authclient.GetApiV1S2sUsersParams{
		Ids:    &ids,
		Active: &active,
		Roles:  &roles,
	}

	resp, err := s.client.GetApiV1S2sUsersWithResponse(ctx, params)
	if err != nil {
		xlog.Error(ctx, "auth check approver request failed", xfield.Error(err))
		return false, fmt.Errorf("%w: call check approver: %w", apperr.ErrAuthUnavailable, err)
	}

	if resp.StatusCode() != http.StatusOK || resp.JSON200 == nil {
		xlog.Error(ctx, "auth check approver returned unexpected status", xfield.Int("status", resp.StatusCode()))
		return false, fmt.Errorf("%w: check approver status %d", apperr.ErrAuthUnavailable, resp.StatusCode())
	}

	users := lo.FromPtr(resp.JSON200.Users)
	return len(users) > 0, nil
}
