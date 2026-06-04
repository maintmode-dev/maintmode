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

// ListActiveUsers fetches active (non-blocked) users from the auth service over
// S2S using the generated auth client. Auth owns the users table; this gateway
// is how maintmode reads it for assignment selection. active=true is always
// sent so blocked users are never returned.
func (s *Gateway) ListActiveUsers(ctx context.Context, q *entity.ListAssignableUsersQuery) (*entity.ListAssignableUsersResult, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "gateway.Auth.ListActiveUsers")
	defer span.End()

	active := true
	params := &authclient.GetApiV1S2sUsersParams{Active: &active}
	if q.Search != "" {
		params.Search = &q.Search
	}
	if q.Role != "" {
		role := string(q.Role)
		params.Role = &role
	}
	if q.Limit > 0 {
		limit := int(q.Limit)
		params.Limit = &limit
	}
	if q.Offset > 0 {
		offset := int(q.Offset)
		params.Offset = &offset
	}

	resp, err := s.client.GetApiV1S2sUsersWithResponse(ctx, params)
	if err != nil {
		xlog.Error(ctx, "auth list users request failed", xfield.Error(err))
		return nil, fmt.Errorf("%w: call list users: %w", apperr.ErrAuthUnavailable, err)
	}

	if resp.StatusCode() != http.StatusOK || resp.JSON200 == nil {
		xlog.Error(ctx, "auth list users returned unexpected status", xfield.Int("status", resp.StatusCode()))
		return nil, fmt.Errorf("%w: list users status %d", apperr.ErrAuthUnavailable, resp.StatusCode())
	}

	return fromS2SUsersResponse(resp.JSON200), nil
}

func fromS2SUsersResponse(body *authclient.ApimodelsListS2SUsersResponse) *entity.ListAssignableUsersResult {
	res := &entity.ListAssignableUsersResult{
		Total:  int64(lo.FromPtr(body.Total)),
		Limit:  int64(lo.FromPtr(body.Limit)),
		Offset: int64(lo.FromPtr(body.Offset)),
	}
	if body.Users == nil {
		return res
	}

	res.Users = lo.Map(*body.Users, func(u authclient.ApimodelsS2SUser, _ int) *entity.User {
		// Id is a string in the generated model; a malformed value yields the
		// zero UUID rather than failing the whole page.
		id, _ := uuid.Parse(lo.FromPtr(u.Id))
		return &entity.User{
			ID:    id,
			Email: lo.FromPtr(u.Email),
			Name:  lo.FromPtr(u.DisplayName),
			Roles: lo.Map(lo.FromPtr(u.Roles), func(r string, _ int) entity.Role {
				return entity.Role(r)
			}),
		}
	})
	return res
}
