package refreshtoken

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) Update(ctx context.Context, t *entity.RefreshToken) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.RefreshToken.Update")
	defer span.End()

	stmt := table.RefreshTokens.
		UPDATE(
			table.RefreshTokens.Revoked,
			table.RefreshTokens.ReplacedBy,
			table.RefreshTokens.GraceTTL,
			table.RefreshTokens.UpdatedAt,
		).
		SET(
			postgres.Bool(t.Revoked),
			pgNullOrDefault(t.ReplacedBy, postgres.String(lo.FromPtr(t.ReplacedBy))),
			pgNullOrDefault(t.GraceTTL, postgres.TimestampzT(lo.FromPtr(t.GraceTTL))),
			postgres.NOW(),
		).
		WHERE(table.RefreshTokens.TokenHash.EQ(postgres.String(t.Token)))

	_, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return err
	}

	return nil
}

func pgNullOrDefault(value, defaultValue any) any {
	if lo.IsNil(value) {
		return postgres.NULL
	}
	return defaultValue
}
