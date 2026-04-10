package refreshtoken

import (
	"context"
	"fmt"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) RevokeByUserID(ctx context.Context, userID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.RefreshToken.RevokeByUserID")
	defer span.End()

	whereExpr := table.RefreshTokens.UserID.EQ(postgres.UUID(userID))

	return s.revoke(ctx, whereExpr)
}

func (s *Store) RevokeFamily(ctx context.Context, family uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.RefreshToken.RevokeFamily")
	defer span.End()

	whereExpr := table.RefreshTokens.Family.EQ(postgres.UUID(family))

	return s.revoke(ctx, whereExpr)
}

func (s *Store) revoke(ctx context.Context, whereExpr postgres.BoolExpression) error {
	if whereExpr == nil {
		return fmt.Errorf("invalid where expression")
	}

	stmt := table.RefreshTokens.
		UPDATE(
			table.RefreshTokens.Revoked,
			table.RefreshTokens.UpdatedAt,
		).
		SET(
			postgres.Bool(true),
			postgres.NOW(),
		).
		WHERE(whereExpr)

	_, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return err
	}

	return nil
}
