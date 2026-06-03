package notifychannel

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// Archive soft-deletes a channel by stamping archived_at. It is idempotent:
// archiving an already-archived or unknown channel succeeds (a non-existent id
// simply updates zero rows).
func (s *Store) Archive(ctx context.Context, channelID uuid.UUID) error {
	return s.setArchivedAt(ctx, "store.MessengerChannels.Archive", channelID, postgres.NOW())
}

func (s *Store) setArchivedAt(ctx context.Context, op string, channelID uuid.UUID, value postgres.Expression) error {
	ctx, span := xlog.WithOperationSpan(ctx, op)
	defer span.End()

	stmt := table.MessengerChannels.
		UPDATE(table.MessengerChannels.ArchivedAt).
		SET(value).
		WHERE(table.MessengerChannels.ID.EQ(postgres.UUID(channelID)))

	if _, err := stmt.ExecContext(ctx, s.db.Executor(ctx)); err != nil {
		return err
	}

	return nil
}
