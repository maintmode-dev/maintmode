package user

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) GetByID(ctx context.Context, userID uuid.UUID) (*entity.User, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.GetByID")
	defer span.End()

	user, err := s.usersStore.GetByID(ctx, userID)
	if err != nil {
		xlog.Error(ctx, "failed to get user", xfield.Error(err))
		return nil, err
	}

	return user, nil
}
