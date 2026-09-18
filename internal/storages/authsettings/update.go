package authsettings

import (
	"context"
	"errors"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// Update writes the flag and its authorship. method and created_* are left
// untouched so the identity and the original row survive.
//
// updated_at is stamped here rather than by the caller, matching the repo's
// other stores: the store owns the modification timestamp, so every write
// advances it and no caller can forget.
func (s *Store) Update(ctx context.Context, setting *entity.AuthMethodSetting) (*entity.AuthMethodSetting, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.AuthSettings.Update")
	defer span.End()

	setting.UpdatedAt = xtime.UTCNow()

	stmt := table.AuthSettings.
		UPDATE(
			table.AuthSettings.Enabled,
			table.AuthSettings.UpdatedAt,
			table.AuthSettings.UpdatedByUserID,
		).
		MODEL(toDB(setting)).
		WHERE(table.AuthSettings.ID.EQ(postgres.UUID(setting.ID))).
		RETURNING(table.AuthSettings.AllColumns)

	updated := new(model.AuthSettings)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), updated); err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, apperr.ErrAuthMethodNotFound
		}

		return nil, err
	}

	return fromDB(updated), nil
}
