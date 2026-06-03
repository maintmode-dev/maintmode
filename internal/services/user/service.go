package user

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/config"

	"github.com/ruko1202/maintmode/internal/services/auditor"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/useridentities"
	"github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Service manages user-related operations including role management.
type Service struct {
	env             config.Environment
	txManager       *dbtx.TxManager
	usersStore      *users.Store
	identitiesStore *useridentities.Store
	auditorSrv      *auditor.Auditor
}

func NewService(
	env config.Environment,
	txManager *dbtx.TxManager,
	usersStore *users.Store,
	identitiesStore *useridentities.Store,
	auditorSrv *auditor.Auditor,
) *Service {
	return &Service{
		env:             env,
		auditorSrv:      auditorSrv,
		txManager:       txManager,
		usersStore:      usersStore,
		identitiesStore: identitiesStore,
	}
}

func (s *Service) updateWithApply(ctx context.Context, userID uuid.UUID, fn func(ctx context.Context, user *entity.User) error) (*entity.User, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.User.applyUserForUpdate")
	defer span.End()

	var user *entity.User
	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		user, err = s.usersStore.GetForUpdateByID(ctx, userID)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}

		if err = fn(ctx, user); err != nil {
			return fmt.Errorf("apply user update: %w", err)
		}

		err = s.usersStore.Update(ctx, user)
		if err != nil {
			return fmt.Errorf("update user: %w", err)
		}

		return nil
	})
	if err != nil {
		xlog.Error(ctx, "failed to update user", xfield.Error(err))
		return nil, err
	}

	return user, nil
}
