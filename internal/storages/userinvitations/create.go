package userinvitations

import (
	"context"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

func (s *Store) Create(ctx context.Context, inv *entity.Invitation) (*entity.Invitation, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.UserInvitations.Create")
	defer span.End()

	row := toDB(inv)

	stmt := table.UserInvitations.
		INSERT(table.UserInvitations.MutableColumns.
			Except(table.UserInvitations.CreatedAt),
		).
		MODEL(row).
		RETURNING(table.UserInvitations.AllColumns)

	err := stmt.QueryContext(ctx, s.db.Executor(ctx), row)
	if err != nil {
		// A unique violation here is the active-pending partial index: there is
		// already a live pending invitation for this email (the service revokes
		// stale pending rows first, so reaching here is a genuine conflict). The
		// only other unique index is on token_hash, whose 256-bit random value
		// makes a collision effectively impossible, so it is not special-cased.
		if dbtx.ErrorIs(err, dbtx.ErrPGUniqueViolation) {
			return nil, apperr.ErrActivePendingExists
		}
		return nil, err
	}

	return fromDB(row), nil
}
