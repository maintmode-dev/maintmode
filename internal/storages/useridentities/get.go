package useridentities

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

func (s *Store) GetByProviderSubject(ctx context.Context, provider entity.AuthMethod, subject string) (*entity.UserIdentity, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserIdentities.GetByProviderSubject")
	defer span.End()

	stmt := table.UserIdentities.
		SELECT(table.UserIdentities.AllColumns).
		WHERE(
			table.UserIdentities.Provider.EQ(postgres.String(string(provider))).
				AND(table.UserIdentities.Subject.EQ(postgres.String(subject))),
		)

	return s.get(ctx, stmt)
}

func (s *Store) GetByUserAndProvider(ctx context.Context, userID uuid.UUID, provider entity.AuthMethod) (*entity.UserIdentity, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserIdentities.GetByUserAndProvider")
	defer span.End()

	stmt := table.UserIdentities.
		SELECT(table.UserIdentities.AllColumns).
		WHERE(
			table.UserIdentities.UserID.EQ(postgres.UUID(userID)).
				AND(table.UserIdentities.Provider.EQ(postgres.String(string(provider)))),
		)

	return s.get(ctx, stmt)
}

func (s *Store) get(ctx context.Context, stmt postgres.Statement) (*entity.UserIdentity, error) {
	row := new(model.UserIdentities)

	err := stmt.QueryContext(ctx, s.db.Executor(ctx), row)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, apperr.ErrProviderNotConnected
		}
		return nil, err
	}

	return fromDB(row), nil
}
