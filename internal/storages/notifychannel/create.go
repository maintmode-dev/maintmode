package notifychannel

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

func (s *Store) Create(ctx context.Context, channel *entity.NotifyChannel) (*entity.NotifyChannel, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.MessengerChannels.Create")
	defer span.End()

	stmt := table.MessengerChannels.
		INSERT(
			table.MessengerChannels.MutableColumns.
				Except(table.MessengerChannels.CreatedAt),
		).
		MODEL(toDB(channel)).
		RETURNING(table.MessengerChannels.AllColumns)

	inserted := new(model.MessengerChannels)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), inserted); err != nil {
		if dbtx.ErrorIs(err, dbtx.ErrPGUniqueViolation) {
			return nil, apperr.ErrNotifyChannelAlreadyExists
		}
		xlog.Error(ctx, "create channel failed", xfield.Error(err))
		return nil, err
	}

	return fromDB(inserted), nil
}
