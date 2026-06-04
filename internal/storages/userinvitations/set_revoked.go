package userinvitations

import (
	"context"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// SetRevoked transitions an invitation to revoked, invalidating its link.
func (s *Store) SetRevoked(ctx context.Context, id uuid.UUID, revokedAt time.Time) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserInvitations.SetRevoked")
	defer span.End()

	stmt := table.UserInvitations.
		UPDATE(
			table.UserInvitations.Status,
			table.UserInvitations.RevokedAt,
		).
		SET(
			postgres.String(string(entity.InvitationStatusRevoked)),
			postgres.TimestampzT(revokedAt),
		).
		WHERE(table.UserInvitations.ID.EQ(postgres.UUID(id)))

	_, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	return err
}
