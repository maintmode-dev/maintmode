package refreshtoken

import (
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
)

func fromDBUser(t *model.RefreshTokens) *entity.RefreshToken {
	return &entity.RefreshToken{
		Token:      t.TokenHash,
		UserID:     t.UserID,
		Family:     t.Family,
		ExpiresAt:  t.ExpiresAt,
		GraceTTL:   t.GraceTTL,
		Revoked:    t.Revoked,
		ReplacedBy: t.ReplacedBy,
		BoundIP:    t.BoundIP,
		CreatedAt:  t.CreatedAt,
		UpdatedAt:  lo.FromPtr(t.UpdatedAt),
	}
}

func toDBRefreshToken(t *entity.RefreshToken) *model.RefreshTokens {
	return &model.RefreshTokens{
		TokenHash:  t.Token,
		UserID:     t.UserID,
		Family:     t.Family,
		ExpiresAt:  t.ExpiresAt,
		GraceTTL:   t.GraceTTL,
		Revoked:    t.Revoked,
		ReplacedBy: t.ReplacedBy,
		BoundIP:    t.BoundIP,
	}
}
