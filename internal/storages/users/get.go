package users

import (
	"context"
	"errors"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) GetByID(ctx context.Context, userID uuid.UUID) (*entity.User, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Users.GetByID")
	defer span.End()

	stmt := table.Users.
		SELECT(table.Users.AllColumns).
		WHERE(table.Users.ID.EQ(postgres.UUID(userID)))

	return s.get(ctx, stmt)
}

func (s *Store) GetForUpdateByID(ctx context.Context, userID uuid.UUID) (*entity.User, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Users.GetForUpdateByID")
	defer span.End()

	stmt := table.Users.
		SELECT(table.Users.AllColumns).
		WHERE(table.Users.ID.EQ(postgres.UUID(userID))).
		FOR(postgres.UPDATE())

	return s.get(ctx, stmt)
}

func (s *Store) GetByOAuthProviderID(ctx context.Context, oauthProviderID string) (*entity.User, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Users.GetByOAuthProviderID")
	defer span.End()

	stmt := table.Users.
		SELECT(table.Users.AllColumns).
		WHERE(table.Users.OAuthProviderID.EQ(postgres.String(oauthProviderID)))

	return s.get(ctx, stmt)
}

func (s *Store) get(ctx context.Context, stmt postgres.Statement) (*entity.User, error) {
	user := new(model.Users)

	err := stmt.QueryContext(ctx, s.db.Executor(ctx), user)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, apperr.ErrUserNotFound
		}
		return nil, err
	}

	return fromDBUser(user), nil
}
