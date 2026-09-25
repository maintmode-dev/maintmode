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

// GetByMethodSubject resolves the identity a method's subject belongs to.
//
// This is the lookup every sign-in makes: the subject comes from the provider,
// and the row it matches decides which user is being signed in.
func (s *Store) GetByMethodSubject(ctx context.Context, ref entity.SignInMethodRef, subject string) (*entity.UserIdentity, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserIdentities.GetByMethodSubject")
	defer span.End()

	stmt := table.UserIdentities.
		SELECT(table.UserIdentities.AllColumns).
		WHERE(
			methodFilter(ref).
				AND(table.UserIdentities.Subject.EQ(postgres.String(subject))),
		)

	return s.get(ctx, stmt)
}

// GetByUserAndMethod returns the identity linking userID to one method.
func (s *Store) GetByUserAndMethod(ctx context.Context, userID uuid.UUID, ref entity.SignInMethodRef) (*entity.UserIdentity, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserIdentities.GetByUserAndMethod")
	defer span.End()

	stmt := table.UserIdentities.
		SELECT(table.UserIdentities.AllColumns).
		WHERE(
			table.UserIdentities.UserID.EQ(postgres.UUID(userID)).
				AND(methodFilter(ref)),
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
