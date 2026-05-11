package stuboauth

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/apperr"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func (p *Service) VerifyToken(ctx context.Context, token string) (*entity.OAuthIDTokenClaims, error) {
	_, span := xlog.WithOperationSpan(ctx, "service.OAuth.Stub.VerifyToken")
	defer span.End()

	if token == "this-is-not-a-valid-jwt" {
		return nil, apperr.ErrInvalidAccessToken
	}

	id := xuuid.NewString()
	return &entity.OAuthIDTokenClaims{
		Subject: xuuid.NewString(),
		Email:   fmt.Sprintf("%s@mail.com", id),
		Name:    fmt.Sprintf("User Name[%s]", id),
	}, nil
}
