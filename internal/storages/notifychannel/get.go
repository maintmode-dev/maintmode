package notifychannel

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// Get resolves a channel by its catalog id (the DB row UUID). Archived channels
// are still returned: subscription validation must keep working for channels
// that have been hidden from the picker.
func (s *Store) Get(ctx context.Context, channelID uuid.UUID) (*entity.NotifyChannel, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.MessengerChannels.Get")
	defer span.End()

	stmt := table.MessengerChannels.
		SELECT(table.MessengerChannels.AllColumns).
		WHERE(table.MessengerChannels.ID.EQ(postgres.UUID(channelID)))

	m := new(model.MessengerChannels)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), m); err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s", apperr.ErrNotFound, channelID)
		}
		return nil, err
	}

	return fromDB(m), nil
}
