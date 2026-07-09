package datakey

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// UpdateWrap re-points a DEK row at a new wrapped-bytes / kek_id pair. Rotation
// uses it after re-wrapping a DEK under a new KEK; only the envelope and the KEK
// id change (the DEK plaintext is unchanged), so nothing that references the row
// needs re-encrypting.
func (s *Store) UpdateWrap(ctx context.Context, id uuid.UUID, encryptedDEK []byte, kekID string) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.DataKeys.UpdateWrap")
	defer span.End()

	stmt := table.DataKeys.
		UPDATE(table.DataKeys.EncryptedDek, table.DataKeys.KekID).
		SET(
			table.DataKeys.EncryptedDek.SET(postgres.Bytea(encryptedDEK)),
			table.DataKeys.KekID.SET(postgres.String(kekID)),
		).
		WHERE(table.DataKeys.ID.EQ(postgres.UUID(id)))

	res, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		xlog.Error(ctx, "update data key wrap failed", xfield.Error(err))
		return err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperr.ErrDataKeyNotFound
	}

	return nil
}
