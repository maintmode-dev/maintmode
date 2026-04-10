package refreshtoken

import (
	"context"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) Save(ctx context.Context, t *entity.RefreshToken) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.RefreshToken.Save")
	defer span.End()

	token := toDBRefreshToken(t)

	stmt := table.RefreshTokens.
		INSERT(table.RefreshTokens.AllColumns.
			Except(table.RefreshTokens.CreatedAt),
		).
		MODEL(token)

	_, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return err
	}

	return nil
}
