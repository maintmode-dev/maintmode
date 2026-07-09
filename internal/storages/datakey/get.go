package datakey

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

// GetByID reads a single data_keys row. Not-found is ErrDataKeyNotFound. The
// integration service uses it to load the wrapped DEK for an existing setting.
func (s *Store) GetByID(ctx context.Context, id uuid.UUID) (*entity.DataKey, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.DataKeys.GetByID")
	defer span.End()

	stmt := table.DataKeys.
		SELECT(table.DataKeys.AllColumns).
		WHERE(table.DataKeys.ID.EQ(postgres.UUID(id)))

	row := new(model.DataKeys)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), row); err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, apperr.ErrDataKeyNotFound
		}
		return nil, err
	}
	return fromDB(row), nil
}
