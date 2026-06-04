package userinvitations

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

func (s *Store) GetByID(ctx context.Context, id uuid.UUID) (*entity.Invitation, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserInvitations.GetByID")
	defer span.End()

	stmt := table.UserInvitations.
		SELECT(table.UserInvitations.AllColumns).
		WHERE(table.UserInvitations.ID.EQ(postgres.UUID(id)))

	return s.get(ctx, stmt)
}

// GetForUpdateByID locks the invitation row for the duration of the
// transaction. Used by revoke/resend so the status check and the mutation are
// atomic against concurrent admins.
func (s *Store) GetForUpdateByID(ctx context.Context, id uuid.UUID) (*entity.Invitation, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserInvitations.GetForUpdateByID")
	defer span.End()

	stmt := table.UserInvitations.
		SELECT(table.UserInvitations.AllColumns).
		WHERE(table.UserInvitations.ID.EQ(postgres.UUID(id))).
		FOR(postgres.UPDATE())

	return s.get(ctx, stmt)
}

// GetByTokenHash resolves an invitation by the sha256 of its raw token. Used by
// the public preview and accept endpoints.
func (s *Store) GetByTokenHash(ctx context.Context, tokenHash string) (*entity.Invitation, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserInvitations.GetByTokenHash")
	defer span.End()

	stmt := table.UserInvitations.
		SELECT(table.UserInvitations.AllColumns).
		WHERE(table.UserInvitations.TokenHash.EQ(postgres.String(tokenHash)))

	return s.get(ctx, stmt)
}

// GetActivePendingByEmail returns the single pending invitation for an email
// (case-insensitive), locked FOR UPDATE. There is at most one such row thanks
// to the active-pending partial unique index. Returns apperr.ErrInvitationNotFound
// when none exists. Note it returns stored-pending rows regardless of expiry —
// the caller decides whether an expired-pending row should be revoked.
func (s *Store) GetActivePendingByEmail(ctx context.Context, email string) (*entity.Invitation, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserInvitations.GetActivePendingByEmail")
	defer span.End()

	stmt := table.UserInvitations.
		SELECT(table.UserInvitations.AllColumns).
		WHERE(
			postgres.LOWER(table.UserInvitations.Email).EQ(postgres.LOWER(postgres.String(email))).
				AND(table.UserInvitations.Status.EQ(
					postgres.String(string(entity.InvitationStatusPending)),
				)),
		).
		FOR(postgres.UPDATE())

	return s.get(ctx, stmt)
}

func (s *Store) get(ctx context.Context, stmt postgres.Statement) (*entity.Invitation, error) {
	row := new(model.UserInvitations)

	err := stmt.QueryContext(ctx, s.db.Executor(ctx), row)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, apperr.ErrInvitationNotFound
		}
		return nil, err
	}

	return fromDB(row), nil
}
