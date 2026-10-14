package refreshtoken

import (
	"context"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// Rotate revokes the token in favor of its successor and opens its grace
// window, but only while the row is still live. A token already revoked --
// rotated by a concurrent refresh, or ended -- is left as it is and
// apperr.ErrRefreshTokenAlreadyRotated returned, so one token never gets two
// successors.
func (s *Store) Rotate(ctx context.Context, tokenHash, successorHash string, graceUntil time.Time) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.RefreshToken.Rotate")
	defer span.End()

	stmt := table.RefreshTokens.
		UPDATE(
			table.RefreshTokens.Revoked,
			table.RefreshTokens.ReplacedBy,
			table.RefreshTokens.GraceTTL,
			table.RefreshTokens.UpdatedAt,
		).
		SET(
			postgres.Bool(true),
			postgres.String(successorHash),
			postgres.TimestampzT(graceUntil),
			postgres.NOW(),
		).
		WHERE(
			table.RefreshTokens.TokenHash.EQ(postgres.String(tokenHash)).
				AND(table.RefreshTokens.Revoked.IS_FALSE()),
		)

	res, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rotate refresh token: %w", err)
	}
	if rows == 0 {
		return apperr.ErrRefreshTokenAlreadyRotated
	}

	return nil
}
