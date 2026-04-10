package refreshtoken

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
)

func (s *Store) GetByTokenHash(ctx context.Context, tokenHash string) (*entity.RefreshToken, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.RefreshToken.GetByTokenHash")
	defer span.End()

	stmt := table.RefreshTokens.
		SELECT(table.RefreshTokens.AllColumns).
		WHERE(table.RefreshTokens.TokenHash.EQ(postgres.String(tokenHash)))

	return s.get(ctx, stmt)
}

func (s *Store) GetByTokenHashForUpdate(ctx context.Context, tokenHash string) (*entity.RefreshToken, error) {
	ctx = xlog.WithOperation(ctx, "store.RefreshToken.GetByTokenHashForUpdate")

	stmt := table.RefreshTokens.
		SELECT(table.RefreshTokens.AllColumns).
		WHERE(table.RefreshTokens.TokenHash.EQ(postgres.String(tokenHash))).
		FOR(postgres.UPDATE())

	return s.get(ctx, stmt)
}

func (s *Store) get(ctx context.Context, stmt postgres.Statement) (*entity.RefreshToken, error) {
	token := new(model.RefreshTokens)

	err := stmt.QueryContext(ctx, s.db.Executor(ctx), token)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, apperr.ErrRefreshTokenNotFound
		}
		return nil, err
	}

	return fromDBUser(token), nil
}
