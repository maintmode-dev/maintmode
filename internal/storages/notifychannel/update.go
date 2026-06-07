package notifychannel

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// Update persists the editable fields of a channel and returns the stored row.
// The caller passes a fully merged entity (read-modify-write); Update stamps
// updated_at to now and writes only the editable columns plus the authorship
// metadata. Transport is intentionally never written: it is immutable.
//
// Writing a fixed column set (not MutableColumns) avoids clobbering archived_at
// owned by the archive/unarchive endpoints (lost update) and keeps created_at /
// created_by_user_id untouched.
func (s *Store) Update(ctx context.Context, ch *entity.NotifyChannel) (*entity.NotifyChannel, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.MessengerChannels.Update")
	defer span.End()

	ch.UpdatedAt = lo.ToPtr(xtime.UTCNow())

	m := toDB(ch)

	stmt := table.MessengerChannels.
		UPDATE(
			table.MessengerChannels.Name,
			table.MessengerChannels.Description,
			table.MessengerChannels.TransportChannelID,
			table.MessengerChannels.UpdatedAt,
			table.MessengerChannels.UpdatedByUserID,
		).
		MODEL(m).
		WHERE(table.MessengerChannels.ID.EQ(postgres.UUID(ch.ID))).
		RETURNING(table.MessengerChannels.AllColumns)

	updated := new(model.MessengerChannels)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), updated); err != nil {
		if dbtx.ErrorIs(err, dbtx.ErrPGUniqueViolation) {
			return nil, apperr.ErrNotifyChannelAlreadyExists
		}
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s", apperr.ErrNotifyChannelNotFound, ch.ID)
		}
		return nil, err
	}

	return fromDB(updated), nil
}
