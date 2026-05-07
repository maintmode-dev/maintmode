package authgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

type introspectRequest struct {
	AccessToken string `json:"access_token"`
}

type introspectResponse struct {
	Active  bool     `json:"active"`
	JTI     string   `json:"jti,omitempty"`
	Subject string   `json:"sub,omitempty"`
	Email   string   `json:"email,omitempty"`
	Roles   []string `json:"roles,omitempty"`
	Exp     int64    `json:"exp,omitempty"`
}

func (s *Service) Introspect(ctx context.Context, tokenString string) (*entity.AccessClaims, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "gateway.Auth.Introspect")
	defer span.End()

	resp, err := s.callIntrospect(ctx, tokenString)
	if err != nil {
		return nil, err
	}
	if !resp.Active {
		return nil, apperr.ErrInvalidAccessToken
	}

	return &entity.AccessClaims{
		Email: resp.Email,
		Roles: lo.Map(resp.Roles, func(item string, _ int) entity.Role {
			return entity.Role(item)
		}),
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        resp.JTI,
			Subject:   resp.Subject,
			ExpiresAt: jwt.NewNumericDate(time.Unix(resp.Exp, 0)),
		},
	}, nil
}

func (s *Service) callIntrospect(ctx context.Context, tokenString string) (*introspectResponse, error) {
	buf := new(bytes.Buffer)

	err := json.NewEncoder(buf).Encode(introspectRequest{AccessToken: tokenString})
	if err != nil {
		return nil, fmt.Errorf("marshal introspect request: %w", err)
	}

	endpoint, err := url.JoinPath(s.baseURL, introspectURI)
	if err != nil {
		return nil, fmt.Errorf("join introspect endpoint: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, buf)
	if err != nil {
		return nil, fmt.Errorf("%w: create introspect request: %w", apperr.ErrAuthUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := s.httpClient.Do(req)
	if err != nil {
		xlog.Error(ctx, "auth introspect request failed", xfield.Error(err))
		return nil, fmt.Errorf("%w: call introspect: %w", apperr.ErrAuthUnavailable, err)
	}
	defer res.Body.Close() //nolint:errcheck

	if res.StatusCode != http.StatusOK {
		xlog.Error(ctx, "auth introspect returned unexpected status", xfield.Int("status", res.StatusCode))
		return nil, fmt.Errorf("%w: introspect status %d", apperr.ErrAuthUnavailable, res.StatusCode)
	}

	result := new(introspectResponse)
	if err := json.NewDecoder(res.Body).Decode(result); err != nil {
		return nil, fmt.Errorf("%w: decode introspect response: %w", apperr.ErrAuthUnavailable, err)
	}

	return result, nil
}
