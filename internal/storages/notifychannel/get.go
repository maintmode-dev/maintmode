package notifychannel

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

func (s *Store) Get(ctx context.Context, channelID uuid.UUID) (*entity.NotifyChannel, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.MessengerChannels.Get")
	defer span.End()

	stmt := table.MessengerChannels.
		SELECT(table.MessengerChannels.AllColumns).
		WHERE(table.MessengerChannels.ID.EQ(postgres.UUID(channelID)))

	return s.get(ctx, stmt)
}

func (s *Store) GetForUpdate(ctx context.Context, channelID uuid.UUID) (*entity.NotifyChannel, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.MessengerChannels.GetForUpdate")
	defer span.End()

	stmt := table.MessengerChannels.
		SELECT(table.MessengerChannels.AllColumns).
		WHERE(table.MessengerChannels.ID.EQ(postgres.UUID(channelID))).
		FOR(postgres.UPDATE())

	return s.get(ctx, stmt)
}

func (s *Store) get(ctx context.Context, stmt postgres.Statement) (*entity.NotifyChannel, error) {
	m := new(model.MessengerChannels)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), m); err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, apperr.ErrNotifyChannelNotFound
		}
		return nil, err
	}

	return fromDB(m), nil
}
