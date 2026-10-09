package refreshtoken

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// RevokeByUserID revokes every session of a user and returns the sessions
// (families) it touched, so the caller can revoke their access tokens too.
func (s *Store) RevokeByUserID(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.RefreshToken.RevokeByUserID")
	defer span.End()

	whereExpr := table.RefreshTokens.UserID.EQ(postgres.UUID(userID))

	return s.revoke(ctx, whereExpr)
}

// RevokeByUserIDExceptFamily revokes every session of a user except one.
//
// It backs a password change made from a live browser: every other session is
// evicted -- the point of changing a password after it leaked -- while the one
// doing the changing survives, because logging someone out of the tab they are
// working in is a surprise, not a security gain.
//
// It returns the sessions (families) it touched, as RevokeByUserID does.
func (s *Store) RevokeByUserIDExceptFamily(ctx context.Context, userID, keep uuid.UUID) ([]uuid.UUID, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.RefreshToken.RevokeByUserIDExceptFamily")
	defer span.End()

	whereExpr := table.RefreshTokens.UserID.EQ(postgres.UUID(userID)).
		AND(table.RefreshTokens.Family.NOT_EQ(postgres.UUID(keep)))

	return s.revoke(ctx, whereExpr)
}

// RevokeFamily revokes every token of a session.
func (s *Store) RevokeFamily(ctx context.Context, family uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.RefreshToken.RevokeFamily")
	defer span.End()

	whereExpr := table.RefreshTokens.Family.EQ(postgres.UUID(family))

	_, err := s.revoke(ctx, whereExpr)

	return err
}

// revoke marks the matched rows revoked and returns the distinct families of
// those rows, read in the same statement so a session created mid-sweep cannot
// be revoked here yet missed by the caller. Families that were already over are
// included; marking one of those again is harmless. Callers that need no
// families ignore them.
func (s *Store) revoke(ctx context.Context, whereExpr postgres.BoolExpression) ([]uuid.UUID, error) {
	stmt := table.RefreshTokens.
		UPDATE(
			table.RefreshTokens.Revoked,
			table.RefreshTokens.UpdatedAt,
		).
		SET(
			postgres.Bool(true),
			postgres.NOW(),
		).
		WHERE(whereExpr).
		// The primary key rides along because jet groups returned rows by it.
		RETURNING(table.RefreshTokens.TokenHash, table.RefreshTokens.Family)

	// A slice destination: no matching rows is an empty result, not qrm.ErrNoRows.
	var rows []model.RefreshTokens
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &rows); err != nil {
		return nil, err
	}

	return lo.Uniq(lo.Map(rows, func(r model.RefreshTokens, _ int) uuid.UUID { return r.Family })), nil
}
