package notifychannel

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
)

// Unarchive clears archived_at, returning a channel to the active catalog. It
// is idempotent: unarchiving an already-active or unknown channel succeeds.
func (s *Store) Unarchive(ctx context.Context, channelID uuid.UUID) error {
	return s.setArchivedAt(ctx, "store.MessengerChannels.Unarchive", channelID, postgres.NULL)
}
