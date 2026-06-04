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

// GetUsersByIDs batch-resolves user profiles by id over S2S, indexed by id. It
// is the no-N+1 author-resolution path: a page of maintenances yields a set of
// author ids resolved in a single auth call. Unlike ListActiveUsers it does NOT
// send active=true — a maintenance may have been authored by a since-blocked or
// removed user, and the read still wants to label them. Ids not present in the
// response are simply absent from the map; the caller decides how to degrade.
func (s *Gateway) GetUsersByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*entity.User, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "gateway.Auth.GetUsersByIDs")
	defer span.End()

	if len(ids) == 0 {
		return map[uuid.UUID]*entity.User{}, nil
	}

	idStrs := lo.Map(ids, func(id uuid.UUID, _ int) string {
		return id.String()
	})
	params := &authclient.GetApiV1S2sUsersParams{Ids: &idStrs}

	resp, err := s.client.GetApiV1S2sUsersWithResponse(ctx, params)
	if err != nil {
		xlog.Error(ctx, "auth get users by ids request failed", xfield.Error(err))
		return nil, fmt.Errorf("%w: call get users by ids: %w", apperr.ErrAuthUnavailable, err)
	}

	if resp.StatusCode() != http.StatusOK || resp.JSON200 == nil {
		xlog.Error(ctx, "auth get users by ids returned unexpected status", xfield.Int("status", resp.StatusCode()))
		return nil, fmt.Errorf("%w: get users by ids status %d", apperr.ErrAuthUnavailable, resp.StatusCode())
	}

	result := fromS2SUsersResponse(resp.JSON200)
	byID := make(map[uuid.UUID]*entity.User, len(result.Users))
	for _, u := range result.Users {
		byID[u.ID] = u
	}
	return byID, nil
}
