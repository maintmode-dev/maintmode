package authgateway

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	authclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/auth"
)

func (s *Gateway) Introspect(ctx context.Context, tokenString string) (*entity.AccessClaims, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "gateway.Auth.Introspect")
	defer span.End()

	resp, err := s.client.PostApiV1S2sIntrospectWithResponse(ctx, authclient.PostApiV1S2sIntrospectJSONRequestBody{
		AccessToken: &tokenString,
	})
	if err != nil {
		xlog.Error(ctx, "auth introspect request failed", xfield.Error(err))
		return nil, fmt.Errorf("%w: call introspect: %w", apperr.ErrAuthUnavailable, err)
	}

	if resp.StatusCode() != http.StatusOK || resp.JSON200 == nil {
		xlog.Error(ctx, "auth introspect returned unexpected status", xfield.Int("status", resp.StatusCode()))
		return nil, fmt.Errorf("%w: introspect status %d", apperr.ErrAuthUnavailable, resp.StatusCode())
	}

	if !lo.FromPtr(resp.JSON200.Active) {
		xlog.Error(ctx, "access token inactive")
		return nil, apperr.ErrInvalidAccessToken
	}

	return fromIntrospectResponse(resp.JSON200), nil
}

func fromIntrospectResponse(body *authclient.ApiauthmodelsIntrospectResponse) *entity.AccessClaims {
	roles := lo.Map(lo.FromPtr(body.Roles), func(item string, _ int) entity.Role {
		return entity.Role(item)
	})

	return &entity.AccessClaims{
		Email: lo.FromPtr(body.Email),
		Roles: roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        lo.FromPtr(body.Jti),
			Subject:   lo.FromPtr(body.Sub),
			ExpiresAt: jwt.NewNumericDate(time.Unix(int64(lo.FromPtr(body.Exp)), 0)),
		},
	}
}
