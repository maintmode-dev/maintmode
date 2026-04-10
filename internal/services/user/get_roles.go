package user

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// GetRoles returns the roles assigned to a user.
func (s *Service) GetRoles(ctx context.Context, userID uuid.UUID) ([]entity.Role, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.GetRoles")
	defer span.End()

	user, err := s.usersStore.GetByID(ctx, userID)
	if err != nil {
		xlog.Error(ctx, "failed to get user", xfield.Error(err))
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user.Roles, nil
}
