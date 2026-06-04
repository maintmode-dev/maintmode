package userinvitations

import (
	"context"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// Resend rotates the token and extends the expiry of a pending invitation.
func (s *Store) Resend(ctx context.Context, id uuid.UUID, tokenHash string, expiresAt, sentAt time.Time) (*entity.Invitation, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserInvitations.Resend")
	defer span.End()

	stmt := table.UserInvitations.
		UPDATE(
			table.UserInvitations.TokenHash,
			table.UserInvitations.ExpiresAt,
			table.UserInvitations.SentAt,
		).
		SET(
			postgres.String(tokenHash),
			postgres.TimestampzT(expiresAt),
			postgres.TimestampzT(sentAt),
		).
		WHERE(table.UserInvitations.ID.EQ(postgres.UUID(id))).
		RETURNING(table.UserInvitations.AllColumns)

	row := new(model.UserInvitations)
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), row)
	if err != nil {
		return nil, err
	}

	return fromDB(row), nil
}

// SendAt marks an invitation as sent at a specific time.
func (s *Store) SendAt(ctx context.Context, id uuid.UUID, sentAt time.Time) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserInvitations.Resend")
	defer span.End()

	stmt := table.UserInvitations.
		UPDATE(
			table.UserInvitations.SentAt,
		).
		SET(
			postgres.TimestampzT(sentAt),
		).
		WHERE(table.UserInvitations.ID.EQ(postgres.UUID(id)))

	_, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	return err
}
